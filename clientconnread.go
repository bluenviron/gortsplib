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
func (cc *ClientConn) Play() (*base.Response, error) {
	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStatePrePlay: {},
	})
	if err != nil {
		return nil, err
	}

	res, err := cc.Do(&base.Request{
		Method: base.Play,
		URL:    cc.streamBaseURL,
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
		cc.rtpInfo = &ri
	}

	cc.state = clientConnStatePlay

	return res, nil
}

// RTPInfo returns the RTP-Info header sent by the server in the PLAY response.
func (cc *ClientConn) RTPInfo() *headers.RTPInfo {
	return cc.rtpInfo
}

func (cc *ClientConn) backgroundPlayUDP() error {
	// open the firewall by sending packets to the counterpart
	for _, cct := range cc.tracks {
		cct.udpRTPListener.write(
			[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

		cct.udpRTCPListener.write(
			[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
	}

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

	reportTicker := time.NewTicker(cc.conf.receiverReportPeriod)
	defer reportTicker.Stop()

	keepaliveTicker := time.NewTicker(clientConnUDPKeepalivePeriod)
	defer keepaliveTicker.Stop()

	checkStreamInitial := true
	checkStreamTicker := time.NewTicker(cc.conf.InitialUDPReadTimeout)
	defer func() {
		checkStreamTicker.Stop()
	}()

	for {
		select {
		case <-cc.backgroundTerminate:
			cc.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for _, cct := range cc.tracks {
				rr := cct.rtcpReceiver.Report(now)
				cct.udpRTCPListener.write(rr)
			}

		case <-keepaliveTicker.C:
			_, err := cc.Do(&base.Request{
				Method: func() base.Method {
					// the vlc integrated rtsp server requires GET_PARAMETER
					if cc.useGetParameter {
						return base.GetParameter
					}
					return base.Options
				}(),
				// use the stream base URL, otherwise some cameras do not reply
				URL:          cc.streamBaseURL,
				SkipResponse: true,
			})
			if err != nil {
				cc.nconn.SetReadDeadline(time.Now())
				<-readerDone
				return err
			}

		case <-checkStreamTicker.C:
			if checkStreamInitial {
				// check that at least one packet has been received
				inTimeout := func() bool {
					for _, cct := range cc.tracks {
						lft := atomic.LoadInt64(cct.udpRTPListener.lastFrameTime)
						if lft != 0 {
							return false
						}

						lft = atomic.LoadInt64(cct.udpRTCPListener.lastFrameTime)
						if lft != 0 {
							return false
						}
					}
					return true
				}()
				if inTimeout {
					cc.nconn.SetReadDeadline(time.Now())
					<-readerDone
					return liberrors.ErrClientNoUDPPacketsRecently{}
				}

				checkStreamInitial = false
				checkStreamTicker.Stop()
				checkStreamTicker = time.NewTicker(clientConnCheckStreamPeriod)

			} else {
				inTimeout := func() bool {
					now := time.Now()
					for _, cct := range cc.tracks {
						lft := time.Unix(atomic.LoadInt64(cct.udpRTPListener.lastFrameTime), 0)
						if now.Sub(lft) < cc.conf.ReadTimeout {
							return false
						}

						lft = time.Unix(atomic.LoadInt64(cct.udpRTCPListener.lastFrameTime), 0)
						if now.Sub(lft) < cc.conf.ReadTimeout {
							return false
						}
					}
					return true
				}()
				if inTimeout {
					cc.nconn.SetReadDeadline(time.Now())
					<-readerDone
					return liberrors.ErrClientUDPTimeout{}
				}
			}

		case err := <-readerDone:
			return err
		}
	}
}

func (cc *ClientConn) backgroundPlayTCP() error {
	// for some reason, SetReadDeadline() must always be called in the same
	// goroutine, otherwise Read() freezes.
	// therefore, we disable the deadline and perform check with a ticker.
	cc.nconn.SetReadDeadline(time.Time{})

	var lastFrameTime int64

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

			track, ok := cc.tracks[frame.TrackID]
			if !ok {
				continue
			}

			now := time.Now()
			atomic.StoreInt64(&lastFrameTime, now.Unix())
			track.rtcpReceiver.ProcessFrame(now, frame.StreamType, frame.Payload)
			cc.readCB(frame.TrackID, frame.StreamType, frame.Payload)
		}
	}()

	reportTicker := time.NewTicker(cc.conf.receiverReportPeriod)
	defer reportTicker.Stop()

	checkStreamTicker := time.NewTicker(clientConnCheckStreamPeriod)
	defer checkStreamTicker.Stop()

	for {
		select {
		case <-cc.backgroundTerminate:
			cc.nconn.SetReadDeadline(time.Now())
			<-readerDone
			return fmt.Errorf("terminated")

		case <-reportTicker.C:
			now := time.Now()
			for trackID, cct := range cc.tracks {
				r := cct.rtcpReceiver.Report(now)
				cc.nconn.SetWriteDeadline(time.Now().Add(cc.conf.WriteTimeout))
				frame := base.InterleavedFrame{
					TrackID:    trackID,
					StreamType: StreamTypeRTCP,
					Payload:    r,
				}
				frame.Write(cc.bw)
			}

		case <-checkStreamTicker.C:
			inTimeout := func() bool {
				now := time.Now()
				lft := time.Unix(atomic.LoadInt64(&lastFrameTime), 0)
				return now.Sub(lft) >= cc.conf.ReadTimeout
			}()
			if inTimeout {
				cc.nconn.SetReadDeadline(time.Now())
				<-readerDone
				return liberrors.ErrClientTCPTimeout{}
			}

		case err := <-readerDone:
			return err
		}
	}
}
