package gortsplib

import (
	"fmt"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
)

// Announce writes an ANNOUNCE request and reads a Response.
func (c *ConnClient) Announce(u *base.URL, tracks Tracks) (*base.Response, error) {
	err := c.checkState(map[connClientState]struct{}{
		connClientStateInitial: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.Do(&base.Request{
		Method: base.ANNOUNCE,
		URL:    u,
		Header: base.Header{
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Content: tracks.Write(),
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.streamUrl = u
	c.state = connClientStatePreRecord

	return res, nil
}

// Record writes a RECORD request and reads a Response.
// This can be called only after Announce() and Setup().
func (c *ConnClient) Record() (*base.Response, error) {
	err := c.checkState(map[connClientState]struct{}{
		connClientStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.Do(&base.Request{
		Method: base.RECORD,
		URL:    c.streamUrl,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.state = connClientStateRecord
	c.publishOpen = true
	c.backgroundTerminate = make(chan struct{})
	c.backgroundDone = make(chan struct{})

	if *c.streamProtocol == StreamProtocolUDP {
		go c.backgroundRecordUDP()
	} else {
		go c.backgroundRecordTCP()
	}

	return nil, nil
}

func (c *ConnClient) backgroundRecordUDP() {
	defer close(c.backgroundDone)

	defer func() {
		c.publishWriteMutex.Lock()
		defer c.publishWriteMutex.Unlock()
		c.publishOpen = false
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

	reportTicker := time.NewTicker(clientSenderReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-c.backgroundTerminate:
			c.nconn.SetReadDeadline(time.Now())
			<-readerDone
			c.publishError = fmt.Errorf("terminated")
			return

		case <-reportTicker.C:
			c.publishWriteMutex.Lock()
			now := time.Now()
			for trackId := range c.rtcpSenders {
				r := c.rtcpSenders[trackId].Report(now)
				if r != nil {
					c.udpRtcpListeners[trackId].write(r)
				}
			}
			c.publishWriteMutex.Unlock()

		case err := <-readerDone:
			c.publishError = err
			return
		}
	}
}

func (c *ConnClient) backgroundRecordTCP() {
	defer close(c.backgroundDone)

	defer func() {
		c.publishWriteMutex.Lock()
		defer c.publishWriteMutex.Unlock()
		c.publishOpen = false
	}()

	reportTicker := time.NewTicker(clientSenderReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-c.backgroundTerminate:
			return

		case <-reportTicker.C:
			c.publishWriteMutex.Lock()
			now := time.Now()
			for trackId := range c.rtcpSenders {
				r := c.rtcpSenders[trackId].Report(now)
				if r != nil {
					c.nconn.SetWriteDeadline(time.Now().Add(c.d.WriteTimeout))
					frame := base.InterleavedFrame{
						TrackId:    trackId,
						StreamType: StreamTypeRtcp,
						Content:    r,
					}
					frame.Write(c.bw)
				}
			}
			c.publishWriteMutex.Unlock()
		}
	}
}

// WriteFrame writes a frame.
// This can be called only after Record().
func (c *ConnClient) WriteFrame(trackId int, streamType StreamType, content []byte) error {
	c.publishWriteMutex.RLock()
	defer c.publishWriteMutex.RUnlock()

	if !c.publishOpen {
		return c.publishError
	}

	now := time.Now()

	c.rtcpSenders[trackId].OnFrame(now, streamType, content)

	if *c.streamProtocol == StreamProtocolUDP {
		if streamType == StreamTypeRtp {
			return c.udpRtpListeners[trackId].write(content)
		}
		return c.udpRtcpListeners[trackId].write(content)
	}

	c.nconn.SetWriteDeadline(now.Add(c.d.WriteTimeout))
	frame := base.InterleavedFrame{
		TrackId:    trackId,
		StreamType: streamType,
		Content:    content,
	}
	return frame.Write(c.bw)
}
