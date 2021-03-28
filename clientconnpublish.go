package gortsplib

import (
	"fmt"
	"strconv"
	"time"

	psdp "github.com/pion/sdp/v3"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/liberrors"
)

// Announce writes an ANNOUNCE request and reads a Response.
func (cc *ClientConn) Announce(u *base.URL, tracks Tracks) (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
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

	res, err := cc.Do(&base.Request{
		Method: base.Announce,
		URL:    u,
		Header: base.Header{
			"Content-Type": base.HeaderValue{"application/sdp"},
		},
		Body: tracks.Write(),
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientWrongStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage}
	}

	cc.streamURL = u
	cc.state = clientConnStatePreRecord

	return res, nil
}

func (cc *ClientConn) backgroundRecordUDP() {
	defer func() {
		cc.publishWriteMutex.Lock()
		defer cc.publishWriteMutex.Unlock()
		cc.publishOpen = false
	}()

	// disable deadline
	cc.nconn.SetReadDeadline(time.Time{})

	readerDone := make(chan error)
	go func() {
		for {
			var res base.Response
			err := res.Read(cc.br)
			if err != nil {
				readerDone <- err
				return
			}
		}
	}()

	reportTicker := time.NewTicker(cc.conf.senderReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-cc.backgroundTerminate:
			cc.nconn.SetReadDeadline(time.Now())
			<-readerDone
			cc.publishError = fmt.Errorf("terminated")
			return

		case <-reportTicker.C:
			cc.publishWriteMutex.Lock()
			now := time.Now()
			for trackID := range cc.rtcpSenders {
				r := cc.rtcpSenders[trackID].Report(now)
				if r != nil {
					cc.udpRTCPListeners[trackID].write(r)
				}
			}
			cc.publishWriteMutex.Unlock()

		case err := <-readerDone:
			cc.publishError = err
			return
		}
	}
}

func (cc *ClientConn) backgroundRecordTCP() {
	defer func() {
		cc.publishWriteMutex.Lock()
		defer cc.publishWriteMutex.Unlock()
		cc.publishOpen = false
	}()

	reportTicker := time.NewTicker(cc.conf.senderReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-cc.backgroundTerminate:
			return

		case <-reportTicker.C:
			cc.publishWriteMutex.Lock()
			now := time.Now()
			for trackID := range cc.rtcpSenders {
				r := cc.rtcpSenders[trackID].Report(now)
				if r != nil {
					cc.nconn.SetWriteDeadline(time.Now().Add(cc.conf.WriteTimeout))
					frame := base.InterleavedFrame{
						TrackID:    trackID,
						StreamType: StreamTypeRTCP,
						Payload:    r,
					}
					frame.Write(cc.bw)
				}
			}
			cc.publishWriteMutex.Unlock()
		}
	}
}

// Record writes a RECORD request and reads a Response.
// This can be called only after Announce() and Setup().
func (cc *ClientConn) Record() (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStatePreRecord: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := cc.Do(&base.Request{
		Method: base.Record,
		URL:    cc.streamURL,
	})
	if err != nil {
		return nil, err
	}

	if res.StatusCode != base.StatusOK {
		return nil, liberrors.ErrClientWrongStatusCode{
			Code: res.StatusCode, Message: res.StatusMessage}
	}

	cc.state = clientConnStateRecord
	cc.publishOpen = true
	cc.backgroundTerminate = make(chan struct{})
	cc.backgroundDone = make(chan struct{})

	go func() {
		defer close(cc.backgroundDone)

		if *cc.streamProtocol == StreamProtocolUDP {
			cc.backgroundRecordUDP()
		} else {
			cc.backgroundRecordTCP()
		}
	}()

	return nil, nil
}

// WriteFrame writes a frame.
// This can be called only after Record().
func (cc *ClientConn) WriteFrame(trackID int, streamType StreamType, payload []byte) error {
	cc.publishWriteMutex.RLock()
	defer cc.publishWriteMutex.RUnlock()

	if !cc.publishOpen {
		return cc.publishError
	}

	now := time.Now()

	cc.rtcpSenders[trackID].ProcessFrame(now, streamType, payload)

	if *cc.streamProtocol == StreamProtocolUDP {
		if streamType == StreamTypeRTP {
			return cc.udpRTPListeners[trackID].write(payload)
		}
		return cc.udpRTCPListeners[trackID].write(payload)
	}

	cc.nconn.SetWriteDeadline(now.Add(cc.conf.WriteTimeout))
	frame := base.InterleavedFrame{
		TrackID:    trackID,
		StreamType: streamType,
		Payload:    payload,
	}
	return frame.Write(cc.bw)
}
