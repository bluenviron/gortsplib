package gortsplib

import (
	"fmt"
	"strconv"
	"time"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/base"
)

// Announce writes an ANNOUNCE request and reads a Response.
func (c *ClientConn) Announce(u *base.URL, tracks Tracks) (*base.Response, error) {
	err := c.checkState(map[clientConnState]struct{}{
		clientConnStateInitial: {},
	})
	if err != nil {
		return nil, err
	}

	// set id, base url and control attribute on tracks
	for i, t := range tracks {
		t.ID = i
		t.BaseURL = u

		t.Media.Attributes = append(t.Media.Attributes, psdp.Attribute{
			Key:   "control",
			Value: "trackID=" + strconv.FormatInt(int64(i), 10),
		})
	}

	res, err := c.Do(&base.Request{
		Method: base.Announce,
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

	c.streamURL = u
	c.state = clientConnStatePreRecord

	return res, nil
}

// Record writes a RECORD request and reads a Response.
// This can be called only after Announce() and Setup().
func (c *ClientConn) Record() (*base.Response, error) {
	err := c.checkState(map[clientConnState]struct{}{
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := c.Do(&base.Request{
		Method: base.Record,
		URL:    c.streamURL,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, fmt.Errorf("bad status code: %d (%s)", res.StatusCode, res.StatusMessage)
	}

	c.state = clientConnStateRecord
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

func (c *ClientConn) backgroundRecordUDP() {
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
			for trackID := range c.rtcpSenders {
				r := c.rtcpSenders[trackID].Report(now)
				if r != nil {
					c.udpRtcpListeners[trackID].write(r)
				}
			}
			c.publishWriteMutex.Unlock()

		case err := <-readerDone:
			c.publishError = err
			return
		}
	}
}

func (c *ClientConn) backgroundRecordTCP() {
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
			for trackID := range c.rtcpSenders {
				r := c.rtcpSenders[trackID].Report(now)
				if r != nil {
					c.nconn.SetWriteDeadline(time.Now().Add(c.c.WriteTimeout))
					frame := base.InterleavedFrame{
						TrackID:    trackID,
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
func (c *ClientConn) WriteFrame(trackID int, streamType StreamType, content []byte) error {
	c.publishWriteMutex.RLock()
	defer c.publishWriteMutex.RUnlock()

	if !c.publishOpen {
		return c.publishError
	}

	now := time.Now()

	c.rtcpSenders[trackID].ProcessFrame(now, streamType, content)

	if *c.streamProtocol == StreamProtocolUDP {
		if streamType == StreamTypeRtp {
			return c.udpRtpListeners[trackID].write(content)
		}
		return c.udpRtcpListeners[trackID].write(content)
	}

	c.nconn.SetWriteDeadline(now.Add(c.c.WriteTimeout))
	frame := base.InterleavedFrame{
		TrackID:    trackID,
		StreamType: streamType,
		Content:    content,
	}
	return frame.Write(c.bw)
}
