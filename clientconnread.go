package gortsplib

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/liberrors"
)

// Play writes a PLAY request and reads a Response.
// This can be called only after Setup().
func (c *ClientConn) Play() (*base.Response, error) {
	err := c.checkState(map[clientConnState]struct{}{
		clientConnStatePrePlay: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.Do(&base.Request{
		Method: base.Play,
		URL:    c.streamURL,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientWrongStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage}
	}

	if v, ok := res.Header["RTP-Info"]; ok {
		var ri headers.RTPInfo
		err := ri.Read(v)
		if err != nil {
			return nil, liberrors.ErrClientRTPInfoInvalid{Err: err}
		}
		c.rtpInfo = &ri
	}

	return res, nil
}

// RTPInfo returns the RTP-Info header sent by the server in the PLAY response.
func (c *ClientConn) RTPInfo() *headers.RTPInfo {
	return c.rtpInfo
}

func (c *ClientConn) backgroundPlayUDP() error {
	// open the firewall by sending packets to the counterpart
	for trackID := range c.udpRTPListeners {
		c.udpRTPListeners[trackID].write(
			[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

		c.udpRTCPListeners[trackID].write(
			[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
	}

	for trackID := range c.udpRTPListeners {
		c.udpRTPListeners[trackID].start()
		c.udpRTCPListeners[trackID].start()
	}

	defer func() {
		for trackID := range c.udpRTPListeners {
			c.udpRTPListeners[trackID].stop()
			c.udpRTCPListeners[trackID].stop()
		}
	}()

	// disable deadline
	c.nconn.SetReadDeadline(time.Time{})

	readerDone := make(chan error)
	go func() {
		for {
			var res base.Response
			err := res.Read(c.br)
			if err != nil {
				readerDone <- err
				return
			}
		}
	}()

	reportTicker := time.NewTicker(clientConnReceiverReportPeriod)
	defer reportTicker.Stop()

	keepaliveTicker := time.NewTicker(clientConnUDPKeepalivePeriod)
	defer keepaliveTicker.Stop()

	checkStreamTicker := time.NewTicker(clientConnUDPCheckStreamPeriod)
	defer checkStreamTicker.Stop()

	for {
		select {
		case <-c.backgroundTerminate:
			c.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID := range c.rtcpReceivers {
				r := c.rtcpReceivers[trackID].Report(now)
				c.udpRTCPListeners[trackID].write(r)
			}

		case <-keepaliveTicker.C:
			_, err := c.Do(&base.Request{
				Method: func() base.Method {
					// the vlc integrated rtsp server requires GET_PARAMETER
					if c.getParameterSupported {
						return base.GetParameter
					}
					return base.Options
				}(),
				// use the stream path, otherwise some cameras do not reply
				URL:          c.streamURL,
				SkipResponse: true,
			})
			if err != nil {
				c.nconn.SetReadDeadline(time.Now())
				<-readerDone
				return err
			}

		case <-checkStreamTicker.C:
			inTimeout := func() bool {
				now := time.Now()
				for trackID := range c.udpRTPListeners {
					last := time.Unix(atomic.LoadInt64(c.udpRTPListeners[trackID].lastFrameTime), 0)
					if now.Sub(last) < c.conf.ReadTimeout {
						return false
					}

					last = time.Unix(atomic.LoadInt64(c.udpRTCPListeners[trackID].lastFrameTime), 0)
					if now.Sub(last) < c.conf.ReadTimeout {
						return false
					}
				}
				return true
			}()
			if inTimeout {
				c.nconn.SetReadDeadline(time.Now())
				<-readerDone
				return liberrors.ErrClientNoUDPPacketsRecently{}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (c *ClientConn) backgroundPlayTCP() error {
	readerDone := make(chan error)
	go func() {
		for {
			frame := base.InterleavedFrame{
				Payload: c.tcpFrameBuffer.Next(),
			}
			err := frame.Read(c.br)
			if err != nil {
				readerDone <- err
				return
			}

			c.rtcpReceivers[frame.TrackID].ProcessFrame(time.Now(), frame.StreamType, frame.Payload)
			c.readCB(frame.TrackID, frame.StreamType, frame.Payload)
		}
	}()

	reportTicker := time.NewTicker(clientConnReceiverReportPeriod)
	defer reportTicker.Stop()

	// for some reason, SetReadDeadline() must always be called in the same
	// goroutine, otherwise Read() freezes.
	// therefore, we call it with a ticker.
	deadlineTicker := time.NewTicker(1 * time.Second)
	defer deadlineTicker.Stop()

	for {
		select {
		case <-deadlineTicker.C:
			c.nconn.SetReadDeadline(time.Now().Add(c.conf.ReadTimeout))

		case <-c.backgroundTerminate:
			c.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID := range c.rtcpReceivers {
				r := c.rtcpReceivers[trackID].Report(now)
				c.nconn.SetWriteDeadline(time.Now().Add(c.conf.WriteTimeout))
				frame := base.InterleavedFrame{
					TrackID:    trackID,
					StreamType: StreamTypeRTCP,
					Payload:    r,
				}
				frame.Write(c.bw)
			}

		case err := <-readerDone:
			return err
		}
	}
}

// ReadFrames starts reading frames.
// it returns a channel that is written when the reading stops.
// This can be called only after Play().
func (c *ClientConn) ReadFrames(onFrame func(int, StreamType, []byte)) chan error {
	// channel is buffered, since listening to it is not mandatory
	done := make(chan error, 1)

	err := c.checkState(map[clientConnState]struct{}{
		clientConnStatePrePlay: {},
	})
	if err != nil {
		done <- err
		return done
	}

	c.state = clientConnStatePlay
	c.readCB = onFrame
	c.backgroundTerminate = make(chan struct{})
	c.backgroundDone = make(chan struct{})

	go func() {
		defer close(c.backgroundDone)

		if *c.streamProtocol == StreamProtocolUDP {
			done <- c.backgroundPlayUDP()
		} else {
			done <- c.backgroundPlayTCP()
		}
	}()

	return done
}
