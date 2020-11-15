package gortsplib

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/base"
)

// Play writes a PLAY request and reads a Response.
// This can be called only after Setup().
func (c *ConnClient) Play() (*base.Response, error) {
	_, err := c.checkState(map[connClientState]struct{}{
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

	if *c.streamProtocol == StreamProtocolUDP {
		c.readFrameFunc = c.readFrameUDP
		c.writeFrameFunc = c.writeFrameUDP
	} else {
		c.readFrameFunc = c.readFrameTCP
		c.writeFrameFunc = c.writeFrameTCP
	}

	c.state.store(connClientStatePlay)

	c.backgroundTerminate = make(chan struct{})
	c.backgroundDone = make(chan struct{})

	if *c.streamProtocol == StreamProtocolUDP {
		c.udpFrame = make(chan base.InterleavedFrame)

		for trackId := range c.udpRtpListeners {
			c.udpRtpListeners[trackId].start()
			c.udpRtcpListeners[trackId].start()
		}

		// open the firewall by sending packets to the counterpart
		for trackId := range c.udpRtpListeners {
			c.WriteFrame(trackId, StreamTypeRtp,
				[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

			c.WriteFrame(trackId, StreamTypeRtcp,
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

	c.nconn.SetReadDeadline(time.Time{}) // disable deadline

	readDone := make(chan error)
	go func() {
		for {
			var res base.Response
			err := res.Read(c.br)
			if err != nil {
				readDone <- err
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
			<-readDone
			c.backgroundUDPError = fmt.Errorf("terminated")
			c.state.store(connClientStateUDPError)
			return

		case <-reportTicker.C:
			for trackId := range c.rtcpReceivers {
				frame := c.rtcpReceivers[trackId].Report()
				c.WriteFrame(trackId, StreamTypeRtcp, frame)
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
				<-readDone
				c.backgroundUDPError = err
				c.state.store(connClientStateUDPError)
				return
			}

		case <-checkStreamTicker.C:
			now := time.Now()

			for _, lastUnix := range c.udpLastFrameTimes {
				last := time.Unix(atomic.LoadInt64(lastUnix), 0)

				if now.Sub(last) >= c.d.ReadTimeout {
					c.nconn.SetReadDeadline(time.Now())
					<-readDone
					c.backgroundUDPError = fmt.Errorf("no packets received recently (maybe there's a firewall/NAT in between)")
					c.state.store(connClientStateUDPError)
					return
				}
			}
		}
	}
}

func (c *ConnClient) backgroundPlayTCP() {
	defer close(c.backgroundDone)

	reportTicker := time.NewTicker(clientReceiverReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-c.backgroundTerminate:
			return

		case <-reportTicker.C:
			for trackId := range c.rtcpReceivers {
				frame := c.rtcpReceivers[trackId].Report()
				c.WriteFrame(trackId, StreamTypeRtcp, frame)
			}
		}
	}
}

func (c *ConnClient) readFrameUDP() (int, StreamType, []byte, error) {
	if c.state.load() != connClientStatePlay {
		return 0, 0, nil, fmt.Errorf("not playing")
	}

	f := <-c.udpFrame
	return f.TrackId, f.StreamType, f.Content, nil
}

func (c *ConnClient) readFrameTCP() (int, StreamType, []byte, error) {
	if c.state.load() != connClientStatePlay {
		return 0, 0, nil, fmt.Errorf("not playing")
	}

	c.nconn.SetReadDeadline(time.Now().Add(c.d.ReadTimeout))
	c.frame.Content = c.tcpFrameBuffer.Next()
	err := c.frame.Read(c.br)
	if err != nil {
		return 0, 0, nil, err
	}

	c.rtcpReceivers[c.frame.TrackId].OnFrame(c.frame.StreamType, c.frame.Content)

	return c.frame.TrackId, c.frame.StreamType, c.frame.Content, nil
}

// ReadFrame reads a frame.
// This can be used only after Play().
func (c *ConnClient) ReadFrame() (int, StreamType, []byte, error) {
	return c.readFrameFunc()
}
