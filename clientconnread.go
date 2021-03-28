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
		URL:    cc.streamURL,
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
	cc.writeFrameAllowed = true

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
					if cc.getParameterSupported {
						return base.GetParameter
					}
					return base.Options
				}(),
				// use the stream path, otherwise some cameras do not reply
				URL:          cc.streamURL,
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
				checkStreamTicker = time.NewTicker(clientConnUDPCheckStreamPeriod)

			} else {
				inTimeout := func() bool {
					now := time.Now()
					for _, cct := range cc.tracks {
						lft := atomic.LoadInt64(cct.udpRTPListener.lastFrameTime)
						if now.Sub(time.Unix(lft, 0)) < cc.conf.ReadTimeout {
							return false
						}

						lft = atomic.LoadInt64(cct.udpRTCPListener.lastFrameTime)
						if now.Sub(time.Unix(lft, 0)) < cc.conf.ReadTimeout {
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

			cc.tracks[frame.TrackID].rtcpReceiver.ProcessFrame(time.Now(), frame.StreamType, frame.Payload)
			cc.readCB(frame.TrackID, frame.StreamType, frame.Payload)
		}
	}()

	reportTicker := time.NewTicker(cc.conf.receiverReportPeriod)
	defer reportTicker.Stop()

	// for some reason, SetReadDeadline() must always be called in the same
	// goroutine, otherwise Read() freezes.
	// therefore, we call it with a ticker.
	deadlineTicker := time.NewTicker(clientConnTCPSetDeadlinePeriod)
	defer deadlineTicker.Stop()

	for {
		select {
		case <-deadlineTicker.C:
			cc.nconn.SetReadDeadline(time.Now().Add(cc.conf.ReadTimeout))

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

		case err := <-readerDone:
			return err
		}
	}
}

// ReadFrames starts reading frames.
// it returns a channel that is written when the reading stops.
// This can be called only after Play().
func (cc *ClientConn) ReadFrames(onFrame func(int, StreamType, []byte)) chan error {
	// channel is buffered, since listening to it is not mandatory
	done := make(chan error, 1)

	err := cc.checkState(map[clientConnState]struct{}{
		clientConnStatePlay: {},
	})
	if err != nil {
		done <- err
		return done
	}

	cc.backgroundRunning = true
	cc.backgroundTerminate = make(chan struct{})
	cc.backgroundDone = make(chan struct{})
	cc.readCB = onFrame

	go func() {
		if *cc.streamProtocol == StreamProtocolUDP {
			err := cc.backgroundPlayUDP()
			close(cc.backgroundDone)

			// automatically change protocol in case of timeout
			if _, ok := err.(liberrors.ErrClientNoUDPPacketsRecently); ok {
				if cc.conf.StreamProtocol == nil {
					err := func() error {
						prevURL := cc.streamURL
						prevTracks := cc.tracks
						cc.reset()
						v := StreamProtocolTCP
						cc.streamProtocol = &v

						err := cc.connOpen(prevURL.Scheme, prevURL.Host)
						if err != nil {
							return err
						}

						_, err = cc.Options(prevURL)
						if err != nil {
							cc.Close()
							return err
						}

						for _, track := range prevTracks {
							_, err := cc.Setup(headers.TransportModePlay, track.track, 0, 0)
							if err != nil {
								cc.Close()
								return err
							}
						}

						_, err = cc.Play()
						if err != nil {
							cc.Close()
							return err
						}

						return <-cc.ReadFrames(onFrame)
					}()
					done <- err
				}
			}

			done <- err

		} else {
			defer close(cc.backgroundDone)
			done <- cc.backgroundPlayTCP()
		}
	}()

	return done
}
