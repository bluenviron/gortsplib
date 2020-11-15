package gortsplib

import (
	"fmt"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
)

// Announce writes an ANNOUNCE request and reads a Response.
func (c *ConnClient) Announce(u *base.URL, tracks Tracks) (*base.Response, error) {
	_, err := c.checkState(map[connClientState]struct{}{
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
	*c.state = connClientStatePreRecord

	return res, nil
}

// Record writes a RECORD request and reads a Response.
// This can be called only after Announce() and Setup().
func (c *ConnClient) Record() (*base.Response, error) {
	_, err := c.checkState(map[connClientState]struct{}{
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

	if *c.streamProtocol == StreamProtocolUDP {
		c.writeFrameFunc = c.writeFrameUDP
	} else {
		c.writeFrameFunc = c.writeFrameTCP
	}

	c.state.store(connClientStateRecord)

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

	select {
	case <-c.backgroundTerminate:
		c.nconn.SetReadDeadline(time.Now())
		<-readDone
		c.backgroundError = fmt.Errorf("terminated")
		c.state.store(connClientStateUDPError)
		return

	case err := <-readDone:
		c.backgroundError = err
		c.state.store(connClientStateUDPError)
		return
	}
}

func (c *ConnClient) backgroundRecordTCP() {
	defer close(c.backgroundDone)
}

func (c *ConnClient) writeFrameUDP(trackId int, streamType StreamType, content []byte) error {
	switch c.state.load() {
	case connClientStateUDPError:
		return c.backgroundError

	case connClientStateRecord:

	default:
		return fmt.Errorf("not recording")
	}

	if streamType == StreamTypeRtp {
		return c.udpRtpListeners[trackId].write(content)
	}
	return c.udpRtcpListeners[trackId].write(content)
}

func (c *ConnClient) writeFrameTCP(trackId int, streamType StreamType, content []byte) error {
	if c.state.load() != connClientStateRecord {
		return fmt.Errorf("not recording")
	}

	c.nconn.SetWriteDeadline(time.Now().Add(c.d.WriteTimeout))
	frame := base.InterleavedFrame{
		TrackId:    trackId,
		StreamType: streamType,
		Content:    content,
	}
	return frame.Write(c.bw)
}

// WriteFrame writes a frame.
// This can be used only after Record().
func (c *ConnClient) WriteFrame(trackId int, streamType StreamType, content []byte) error {
	return c.writeFrameFunc(trackId, streamType, content)
}
