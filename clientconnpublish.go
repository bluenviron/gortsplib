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

	cc.ReadFrames(func(trackID int, streamType StreamType, payload []byte) {
	})

	return nil, nil
}

func (cc *ClientConn) backgroundRecordUDP() error {
	for _, cct := range cc.tracks {
		cct.udpRTPListener.start()
		cct.udpRTCPListener.start()
	}

	defer func() {
		for _, cct := range cc.tracks {
			cct.udpRTPListener.stop()
			cct.udpRTCPListener.stop()
		}
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
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range cc.tracks {
				sr := cct.rtcpSender.Report(now)
				if sr != nil {
					cc.WriteFrame(trackID, StreamTypeRTCP, sr)
				}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (cc *ClientConn) backgroundRecordTCP() error {
	// disable deadline
	cc.nconn.SetReadDeadline(time.Time{})

	readerDone := make(chan error)
	go func() {
		for {
			frame := base.InterleavedFrame{
				Payload: cc.tcpFrameBuffer.Next(),
			}
			err := frame.Read(cc.br)
			if err != nil {
				readerDone <- err
				return
			}

			cc.readCB(frame.TrackID, frame.StreamType, frame.Payload)
		}
	}()

	reportTicker := time.NewTicker(cc.conf.senderReportPeriod)
	defer reportTicker.Stop()

	for {
		select {
		case <-cc.backgroundTerminate:
			cc.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range cc.tracks {
				sr := cct.rtcpSender.Report(now)
				if sr != nil {
					cc.WriteFrame(trackID, StreamTypeRTCP, sr)
				}
			}

		case err := <-readerDone:
			return err
		}
	}
}
