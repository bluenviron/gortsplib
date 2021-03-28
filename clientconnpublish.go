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
			cc.writeError = fmt.Errorf("terminated")
			return

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range cc.tracks {
				sr := cct.rtcpSender.Report(now)
				if sr != nil {
					cc.WriteFrame(trackID, StreamTypeRTCP, sr)
				}
			}

		case err := <-readerDone:
			cc.writeError = err
			return
		}
	}
}

func (cc *ClientConn) backgroundRecordTCP() {
	reportTicker := time.NewTicker(cc.conf.senderReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-cc.backgroundTerminate:
			return

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range cc.tracks {
				sr := cct.rtcpSender.Report(now)
				if sr != nil {
					cc.WriteFrame(trackID, StreamTypeRTCP, sr)
				}
			}
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
	cc.writeFrameAllowed = true

	cc.backgroundRunning = true
	cc.backgroundTerminate = make(chan struct{})
	cc.backgroundDone = make(chan struct{})

	go func() {
		defer close(cc.backgroundDone)

		defer func() {
			cc.writeMutex.Lock()
			defer cc.writeMutex.Unlock()
			cc.writeFrameAllowed = false
		}()

		if *cc.streamProtocol == StreamProtocolUDP {
			cc.backgroundRecordUDP()
		} else {
			cc.backgroundRecordTCP()
		}
	}()

	return nil, nil
}
