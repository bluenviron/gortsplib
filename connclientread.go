package gortsplib

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
)

// Play writes a PLAY request and reads a Response.
// This can be called only after Setup().
func (c *ConnClient) Play() (*base.Response, error) {
	err := c.checkState(map[connClientState]struct{}{
		connClientStatePrePlay: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.Do(&base.Request{
		Method: base.PLAY,
		URL:    c.streamUrl,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.state = connClientStatePlay

	c.readFrame = make(chan base.InterleavedFrame)
	c.backgroundTerminate = make(chan struct{})
	c.backgroundDone = make(chan struct{})

	if *c.streamProtocol == StreamProtocolUDP {
		// open the firewall by sending packets to the counterpart
		for trackId := range c.udpRtpListeners {
			c.udpRtpListeners[trackId].write(
				[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

			c.udpRtcpListeners[trackId].write(
				[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
		}

		go c.backgroundPlayUDP()
	} else {
		go c.backgroundPlayTCP()
	}

	return res, nil
}

func (c *ConnClient) backgroundPlayUDP() {
	defer close(c.backgroundDone)

	defer func() {
		ch := c.readFrame
		go func() {
			for range ch {
			}
		}()

		for trackId := range c.udpRtpListeners {
			c.udpRtpListeners[trackId].stop()
			c.udpRtcpListeners[trackId].stop()
		}

		close(ch)
	}()

	for trackId := range c.udpRtpListeners {
		c.udpRtpListeners[trackId].start()
		c.udpRtcpListeners[trackId].start()
	}

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

	reportTicker := time.NewTicker(clientReceiverReportPeriod)
	defer reportTicker.Stop()

	keepaliveTicker := time.NewTicker(clientUDPKeepalivePeriod)
	defer keepaliveTicker.Stop()

	checkStreamTicker := time.NewTicker(clientUDPCheckStreamPeriod)
	defer checkStreamTicker.Stop()

	for {
		select {
		case <-c.backgroundTerminate:
			c.nconn.SetReadDeadline(time.Now())
			<-readerDone
			c.backgroundError = fmt.Errorf("terminated")
			return

		case <-reportTicker.C:
			for trackId := range c.rtcpReceivers {
				report := c.rtcpReceivers[trackId].Report()
				c.udpRtcpListeners[trackId].write(report)
			}

		case <-keepaliveTicker.C:
			_, err := c.Do(&base.Request{
				Method: func() base.Method {
					// the vlc integrated rtsp server requires GET_PARAMETER
					if c.getParameterSupported {
						return base.GET_PARAMETER
					}
					return base.OPTIONS
				}(),
				// use the stream path, otherwise some cameras do not reply
				URL:          c.streamUrl,
				SkipResponse: true,
			})
			if err != nil {
				c.nconn.SetReadDeadline(time.Now())
				<-readerDone
				c.backgroundError = err
				return
			}

		case <-checkStreamTicker.C:
			now := time.Now()

			for _, lastUnix := range c.udpLastFrameTimes {
				last := time.Unix(atomic.LoadInt64(lastUnix), 0)

				if now.Sub(last) >= c.d.ReadTimeout {
					c.nconn.SetReadDeadline(time.Now())
					<-readerDone
					c.backgroundError = fmt.Errorf("no packets received recently (maybe there's a firewall/NAT in between)")
					return
				}
			}

		case err := <-readerDone:
			c.backgroundError = err
			return
		}
	}
}

func (c *ConnClient) backgroundPlayTCP() {
	defer close(c.backgroundDone)

	defer func() {
		ch := c.readFrame
		go func() {
			for range ch {
			}
		}()
		close(ch)
	}()

	readerDone := make(chan error)
	go func() {
		for {
			c.nconn.SetReadDeadline(time.Now().Add(c.d.ReadTimeout))
			frame := base.InterleavedFrame{
				Content: c.tcpFrameBuffer.Next(),
			}
			err := frame.Read(c.br)
			if err != nil {
				readerDone <- err
				return
			}

			c.rtcpReceivers[frame.TrackId].OnFrame(frame.StreamType, frame.Content)

			c.readFrame <- frame
		}
	}()

	reportTicker := time.NewTicker(clientReceiverReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-c.backgroundTerminate:
			c.nconn.SetReadDeadline(time.Now())
			<-readerDone
			c.backgroundError = fmt.Errorf("terminated")
			return

		case <-reportTicker.C:
			for trackId := range c.rtcpReceivers {
				report := c.rtcpReceivers[trackId].Report()
				c.nconn.SetWriteDeadline(time.Now().Add(c.d.WriteTimeout))
				frame := base.InterleavedFrame{
					TrackId:    trackId,
					StreamType: StreamTypeRtcp,
					Content:    report,
				}
				frame.Write(c.bw)
			}

		case err := <-readerDone:
			c.backgroundError = err
			return
		}
	}
}

// ReadFrame reads a frame.
// This can be used only after Play().
func (c *ConnClient) ReadFrame() (int, StreamType, []byte, error) {
	f, ok := <-c.readFrame
	if !ok {
		return 0, 0, nil, c.backgroundError
	}

	return f.TrackId, f.StreamType, f.Content, nil
}
