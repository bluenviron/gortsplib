package gortsplib

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/liberrors"
	"github.com/aler9/gortsplib/pkg/rtcpreceiver"
)

const (
	serverSessionCheckStreamPeriod = 1 * time.Second
)

func setupGetTrackIDPathQuery(url *base.URL,
	thMode *headers.TransportMode,
	announcedTracks []ServerSessionAnnouncedTrack,
	setupPath *string, setupQuery *string) (int, string, string, error) {

	pathAndQuery, ok := url.RTSPPathAndQuery()
	if !ok {
		return 0, "", "", liberrors.ErrServerNoPath{}
	}

	if thMode == nil || *thMode == headers.TransportModePlay {
		i := stringsReverseIndex(pathAndQuery, "/trackID=")

		// URL doesn't contain trackID - it's track zero
		if i < 0 {
			if !strings.HasSuffix(pathAndQuery, "/") {
				return 0, "", "", fmt.Errorf("path must end with a slash (%v)", pathAndQuery)
			}
			pathAndQuery = pathAndQuery[:len(pathAndQuery)-1]

			path, query := base.PathSplitQuery(pathAndQuery)

			// we assume it's track 0
			return 0, path, query, nil
		}

		tmp, err := strconv.ParseInt(pathAndQuery[i+len("/trackID="):], 10, 64)
		if err != nil || tmp < 0 {
			return 0, "", "", fmt.Errorf("unable to parse track ID (%v)", pathAndQuery)
		}
		trackID := int(tmp)
		pathAndQuery = pathAndQuery[:i]

		path, query := base.PathSplitQuery(pathAndQuery)

		if setupPath != nil && (path != *setupPath || query != *setupQuery) {
			return 0, "", "", fmt.Errorf("can't setup tracks with different paths")
		}

		return trackID, path, query, nil
	}

	for trackID, track := range announcedTracks {
		u, _ := track.track.URL()
		if u.String() == url.String() {
			return trackID, *setupPath, *setupQuery, nil
		}
	}

	return 0, "", "", fmt.Errorf("invalid track path (%s)", pathAndQuery)
}

// ServerSessionState is a state of a ServerSession.
type ServerSessionState int

// standard states.
const (
	ServerSessionStateInitial ServerSessionState = iota
	ServerSessionStatePrePlay
	ServerSessionStatePlay
	ServerSessionStatePreRecord
	ServerSessionStateRecord
)

// String implements fmt.Stringer.
func (s ServerSessionState) String() string {
	switch s {
	case ServerSessionStateInitial:
		return "initial"
	case ServerSessionStatePrePlay:
		return "prePlay"
	case ServerSessionStatePlay:
		return "play"
	case ServerSessionStatePreRecord:
		return "preRecord"
	case ServerSessionStateRecord:
		return "record"
	}
	return "unknown"
}

// ServerSessionSetuppedTrack is a setupped track of a ServerSession.
type ServerSessionSetuppedTrack struct {
	udpRTPPort  int
	udpRTCPPort int
}

// ServerSessionAnnouncedTrack is an announced track of a ServerSession.
type ServerSessionAnnouncedTrack struct {
	track        *Track
	rtcpReceiver *rtcpreceiver.RTCPReceiver
}

// ServerSession is a server-side RTSP session.
type ServerSession struct {
	s  *Server
	id string
	wg *sync.WaitGroup

	state            ServerSessionState
	setuppedTracks   map[int]ServerSessionSetuppedTrack
	setupProtocol    *StreamProtocol
	setupPath        *string
	setupQuery       *string
	lastRequestTime  time.Time
	linkedConn       *ServerConn                   // tcp
	udpIP            net.IP                        // udp
	udpZone          string                        // udp
	announcedTracks  []ServerSessionAnnouncedTrack // publish
	udpLastFrameTime *int64                        // publish, udp

	// in
	request        chan sessionReq
	innerTerminate chan struct{}
	terminate      chan struct{}
}

func newServerSession(s *Server, id string, wg *sync.WaitGroup) *ServerSession {
	ss := &ServerSession{
		s:               s,
		id:              id,
		wg:              wg,
		lastRequestTime: time.Now(),
		request:         make(chan sessionReq),
		innerTerminate:  make(chan struct{}, 1),
		terminate:       make(chan struct{}),
	}

	wg.Add(1)
	go ss.run()

	return ss
}

// Close closes the ServerSession.
func (ss *ServerSession) Close() error {
	select {
	case ss.innerTerminate <- struct{}{}:
	default:
	}
	return nil
}

// State returns the state of the session.
func (ss *ServerSession) State() ServerSessionState {
	return ss.state
}

// StreamProtocol returns the stream protocol of the setupped tracks.
func (ss *ServerSession) StreamProtocol() *StreamProtocol {
	return ss.setupProtocol
}

// SetuppedTracks returns the setupped tracks.
func (ss *ServerSession) SetuppedTracks() map[int]ServerSessionSetuppedTrack {
	return ss.setuppedTracks
}

// AnnouncedTracks returns the announced tracks.
func (ss *ServerSession) AnnouncedTracks() []ServerSessionAnnouncedTrack {
	return ss.announcedTracks
}

func (ss *ServerSession) checkState(allowed map[ServerSessionState]struct{}) error {
	if _, ok := allowed[ss.state]; ok {
		return nil
	}

	allowedList := make([]fmt.Stringer, len(allowed))
	i := 0
	for a := range allowed {
		allowedList[i] = a
		i++
	}
	return liberrors.ErrServerWrongState{AllowedList: allowedList, State: ss.state}
}

func (ss *ServerSession) run() {
	defer ss.wg.Done()

	if h, ok := ss.s.Handler.(ServerHandlerOnSessionOpen); ok {
		h.OnSessionOpen(ss)
	}

	readDone := make(chan error)
	go func() {
		readDone <- func() error {
			checkTimeoutTicker := time.NewTicker(serverSessionCheckStreamPeriod)
			defer checkTimeoutTicker.Stop()

			receiverReportTicker := time.NewTicker(ss.s.receiverReportPeriod)
			defer receiverReportTicker.Stop()

			for {
				select {
				case req := <-ss.request:
					res, err := ss.handleRequest(req.sc, req.req)

					ss.lastRequestTime = time.Now()

					if res.StatusCode == base.StatusOK {
						if res.Header == nil {
							res.Header = make(base.Header)
						}
						res.Header["Session"] = base.HeaderValue{ss.id}
					}

					if _, ok := err.(liberrors.ErrServerSessionTeardown); ok {
						req.res <- sessionReqRes{res: res, err: nil}
						return liberrors.ErrServerSessionTeardown{}
					}

					req.res <- sessionReqRes{
						res: res,
						err: err,
						ss:  ss,
					}

				case <-checkTimeoutTicker.C:
					switch {
					// in case of record and UDP, timeout happens when no frames are being received
					case ss.state == ServerSessionStateRecord && *ss.setupProtocol == StreamProtocolUDP:
						now := time.Now()
						lft := atomic.LoadInt64(ss.udpLastFrameTime)
						if now.Sub(time.Unix(lft, 0)) >= ss.s.ReadTimeout {
							return liberrors.ErrServerSessionTimedOut{}
						}

					// in case there's a linked TCP connection, timeout is handled in the connection
					case ss.linkedConn != nil:

					// otherwise, timeout happens when no requests arrives
					default:
						now := time.Now()
						if now.Sub(ss.lastRequestTime) >= ss.s.closeSessionAfterNoRequestsFor {
							return liberrors.ErrServerSessionTimedOut{}
						}
					}

				case <-receiverReportTicker.C:
					if ss.state != ServerSessionStateRecord {
						continue
					}

					now := time.Now()
					for trackID, track := range ss.announcedTracks {
						r := track.rtcpReceiver.Report(now)
						ss.WriteFrame(trackID, StreamTypeRTCP, r)
					}

				case <-ss.innerTerminate:
					return liberrors.ErrServerTerminated{}
				}
			}
		}()
	}()

	var err error
	select {
	case err = <-readDone:
		go func() {
			for req := range ss.request {
				req.res <- sessionReqRes{
					res: &base.Response{
						StatusCode: base.StatusBadRequest,
					},
					err: liberrors.ErrServerTerminated{},
				}
			}
		}()

		ss.s.sessionClose <- ss
		<-ss.terminate

	case <-ss.terminate:
		select {
		case ss.innerTerminate <- struct{}{}:
		default:
		}
		<-readDone

		err = liberrors.ErrServerTerminated{}
	}

	switch ss.state {
	case ServerSessionStatePlay:
		if *ss.setupProtocol == StreamProtocolUDP {
			ss.s.udpRTCPListener.removeClient(ss)
		}

	case ServerSessionStateRecord:
		if *ss.setupProtocol == StreamProtocolUDP {
			ss.s.udpRTPListener.removeClient(ss)
			ss.s.udpRTCPListener.removeClient(ss)
		}
	}

	if ss.linkedConn != nil {
		ss.s.connClose <- ss.linkedConn
	}

	close(ss.request)

	if h, ok := ss.s.Handler.(ServerHandlerOnSessionClose); ok {
		h.OnSessionClose(ss, err)
	}
}

func (ss *ServerSession) handleRequest(sc *ServerConn, req *base.Request) (*base.Response, error) {
	if ss.linkedConn != nil && sc != ss.linkedConn {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, liberrors.ErrServerSessionLinkedToOtherConn{}
	}

	switch req.Method {
	case base.Options:
		var methods []string
		if _, ok := sc.s.Handler.(ServerHandlerOnDescribe); ok {
			methods = append(methods, string(base.Describe))
		}
		if _, ok := sc.s.Handler.(ServerHandlerOnAnnounce); ok {
			methods = append(methods, string(base.Announce))
		}
		if _, ok := sc.s.Handler.(ServerHandlerOnSetup); ok {
			methods = append(methods, string(base.Setup))
		}
		if _, ok := sc.s.Handler.(ServerHandlerOnPlay); ok {
			methods = append(methods, string(base.Play))
		}
		if _, ok := sc.s.Handler.(ServerHandlerOnRecord); ok {
			methods = append(methods, string(base.Record))
		}
		if _, ok := sc.s.Handler.(ServerHandlerOnPause); ok {
			methods = append(methods, string(base.Pause))
		}
		methods = append(methods, string(base.GetParameter))
		if _, ok := sc.s.Handler.(ServerHandlerOnSetParameter); ok {
			methods = append(methods, string(base.SetParameter))
		}
		methods = append(methods, string(base.Teardown))

		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Public": base.HeaderValue{strings.Join(methods, ", ")},
			},
		}, nil

	case base.Announce:
		err := ss.checkState(map[ServerSessionState]struct{}{
			ServerSessionStateInitial: {},
		})
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, err
		}

		ct, ok := req.Header["Content-Type"]
		if !ok || len(ct) != 1 {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerContentTypeMissing{}
		}

		if ct[0] != "application/sdp" {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerContentTypeUnsupported{CT: ct}
		}

		tracks, err := ReadTracks(req.Body, req.URL)
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerSDPInvalid{Err: err}
		}

		if len(tracks) == 0 {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerSDPNoTracksDefined{}
		}

		pathAndQuery, ok := req.URL.RTSPPath()
		if !ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerNoPath{}
		}

		path, query := base.PathSplitQuery(pathAndQuery)

		for _, track := range tracks {
			trackURL, err := track.URL()
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("unable to generate track URL")
			}

			trackPath, ok := trackURL.RTSPPath()
			if !ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("invalid track URL (%v)", trackURL)
			}

			if !strings.HasPrefix(trackPath, path) {
				return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, fmt.Errorf("invalid track path: must begin with '%s', but is '%s'",
						path, trackPath)
			}
		}

		res, err := ss.s.Handler.(ServerHandlerOnAnnounce).OnAnnounce(&ServerHandlerOnAnnounceCtx{
			Session: ss,
			Conn:    sc,
			Req:     req,
			Path:    path,
			Query:   query,
			Tracks:  tracks,
		})

		if res.StatusCode == base.StatusOK {
			ss.state = ServerSessionStatePreRecord
			ss.setupPath = &path
			ss.setupQuery = &query

			ss.announcedTracks = make([]ServerSessionAnnouncedTrack, len(tracks))
			for trackID, track := range tracks {
				clockRate, _ := track.ClockRate()
				ss.announcedTracks[trackID] = ServerSessionAnnouncedTrack{
					track:        track,
					rtcpReceiver: rtcpreceiver.New(nil, clockRate),
				}
			}

			v := time.Now().Unix()
			ss.udpLastFrameTime = &v
		}

		return res, err

	case base.Setup:
		err := ss.checkState(map[ServerSessionState]struct{}{
			ServerSessionStateInitial:   {},
			ServerSessionStatePrePlay:   {},
			ServerSessionStatePreRecord: {},
		})
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, err
		}

		var th headers.Transport
		err = th.Read(req.Header["Transport"])
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerTransportHeaderInvalid{Err: err}
		}

		if th.Delivery != nil && *th.Delivery == base.StreamDeliveryMulticast {
			return &base.Response{
				StatusCode: base.StatusUnsupportedTransport,
			}, nil
		}

		trackID, path, query, err := setupGetTrackIDPathQuery(req.URL, th.Mode,
			ss.announcedTracks, ss.setupPath, ss.setupQuery)
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, err
		}

		if _, ok := ss.setuppedTracks[trackID]; ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerTrackAlreadySetup{TrackID: trackID}
		}

		switch ss.state {
		case ServerSessionStateInitial, ServerSessionStatePrePlay: // play
			if th.Mode != nil && *th.Mode != headers.TransportModePlay {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderWrongMode{Mode: th.Mode}
			}

		default: // record
			if th.Mode == nil || *th.Mode != headers.TransportModeRecord {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderWrongMode{Mode: th.Mode}
			}
		}

		if th.Protocol == StreamProtocolUDP {
			if ss.s.udpRTPListener == nil {
				return &base.Response{
					StatusCode: base.StatusUnsupportedTransport,
				}, nil
			}

			if th.ClientPorts == nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderNoClientPorts{}
			}

		} else {
			if th.InterleavedIDs == nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderNoInterleavedIDs{}
			}

			if th.InterleavedIDs[0] != (trackID*2) ||
				th.InterleavedIDs[1] != (1+trackID*2) {
				return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, liberrors.ErrServerTransportHeaderWrongInterleavedIDs{
						Expected: [2]int{(trackID * 2), (1 + trackID*2)}, Value: *th.InterleavedIDs}
			}
		}

		if ss.setupProtocol != nil && *ss.setupProtocol != th.Protocol {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerTracksDifferentProtocols{}
		}

		res, err := ss.s.Handler.(ServerHandlerOnSetup).OnSetup(&ServerHandlerOnSetupCtx{
			Session:   ss,
			Conn:      sc,
			Req:       req,
			Path:      path,
			Query:     query,
			TrackID:   trackID,
			Transport: &th,
		})

		if res.StatusCode == base.StatusOK {
			ss.setupProtocol = &th.Protocol

			if ss.setuppedTracks == nil {
				ss.setuppedTracks = make(map[int]ServerSessionSetuppedTrack)
			}

			if res.Header == nil {
				res.Header = make(base.Header)
			}

			if th.Protocol == StreamProtocolUDP {
				ss.setuppedTracks[trackID] = ServerSessionSetuppedTrack{
					udpRTPPort:  th.ClientPorts[0],
					udpRTCPPort: th.ClientPorts[1],
				}

				res.Header["Transport"] = headers.Transport{
					Protocol: StreamProtocolUDP,
					Delivery: func() *base.StreamDelivery {
						v := base.StreamDeliveryUnicast
						return &v
					}(),
					ClientPorts: th.ClientPorts,
					ServerPorts: &[2]int{sc.s.udpRTPListener.port(), sc.s.udpRTCPListener.port()},
				}.Write()

			} else {
				ss.setuppedTracks[trackID] = ServerSessionSetuppedTrack{}

				res.Header["Transport"] = headers.Transport{
					Protocol:       StreamProtocolTCP,
					InterleavedIDs: th.InterleavedIDs,
				}.Write()
			}
		}

		if ss.state == ServerSessionStateInitial {
			ss.state = ServerSessionStatePrePlay
			ss.setupPath = &path
			ss.setupQuery = &query
		}

		// workaround to prevent a bug in rtspclientsink
		// that makes impossible for the client to receive the response
		// and send frames.
		// this was causing problems during unit tests.
		if ua, ok := req.Header["User-Agent"]; ok && len(ua) == 1 &&
			strings.HasPrefix(ua[0], "GStreamer") {
			select {
			case <-time.After(1 * time.Second):
			case <-sc.terminate:
			}
		}

		return res, err

	case base.Play:
		// play can be sent twice, allow calling it even if we're already playing
		err := ss.checkState(map[ServerSessionState]struct{}{
			ServerSessionStatePrePlay: {},
			ServerSessionStatePlay:    {},
		})
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, err
		}

		if len(ss.setuppedTracks) == 0 {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerNoTracksSetup{}
		}

		pathAndQuery, ok := req.URL.RTSPPath()
		if !ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerNoPath{}
		}

		// path can end with a slash due to Content-Base, remove it
		pathAndQuery = strings.TrimSuffix(pathAndQuery, "/")

		path, query := base.PathSplitQuery(pathAndQuery)

		// allow to use WriteFrame() before response
		if ss.state != ServerSessionStatePlay {
			if *ss.setupProtocol == StreamProtocolUDP {
				ss.udpIP = sc.ip()
				ss.udpZone = sc.zone()
			} else {
				ss.linkedConn = sc
			}
		}

		res, err := sc.s.Handler.(ServerHandlerOnPlay).OnPlay(&ServerHandlerOnPlayCtx{
			Session: ss,
			Conn:    sc,
			Req:     req,
			Path:    path,
			Query:   query,
		})

		if ss.state != ServerSessionStatePlay {
			if res.StatusCode == base.StatusOK {
				ss.state = ServerSessionStatePlay

				if *ss.setupProtocol == StreamProtocolUDP {
					// readers can send RTCP frames, they cannot sent RTP frames
					for trackID, track := range ss.setuppedTracks {
						sc.s.udpRTCPListener.addClient(ss.udpIP, track.udpRTCPPort, ss, trackID, false)
					}

					return res, err
				}

				return res, liberrors.ErrServerTCPFramesEnable{}
			}

			ss.udpIP = nil
			ss.udpZone = ""
			ss.linkedConn = nil
		}

		return res, err

	case base.Record:
		err := ss.checkState(map[ServerSessionState]struct{}{
			ServerSessionStatePreRecord: {},
		})
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, err
		}

		if len(ss.setuppedTracks) != len(ss.announcedTracks) {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerNotAllAnnouncedTracksSetup{}
		}

		pathAndQuery, ok := req.URL.RTSPPath()
		if !ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerNoPath{}
		}

		// path can end with a slash due to Content-Base, remove it
		pathAndQuery = strings.TrimSuffix(pathAndQuery, "/")

		path, query := base.PathSplitQuery(pathAndQuery)

		// allow to use WriteFrame() before response
		if *ss.setupProtocol == StreamProtocolUDP {
			ss.udpIP = sc.ip()
			ss.udpZone = sc.zone()
		} else {
			ss.linkedConn = sc
		}

		res, err := ss.s.Handler.(ServerHandlerOnRecord).OnRecord(&ServerHandlerOnRecordCtx{
			Session: ss,
			Conn:    sc,
			Req:     req,
			Path:    path,
			Query:   query,
		})

		if res.StatusCode == base.StatusOK {
			ss.state = ServerSessionStateRecord

			if *ss.setupProtocol == StreamProtocolUDP {
				for trackID, track := range ss.setuppedTracks {
					ss.s.udpRTPListener.addClient(ss.udpIP, track.udpRTPPort, ss, trackID, true)
					ss.s.udpRTCPListener.addClient(ss.udpIP, track.udpRTCPPort, ss, trackID, true)

					// open the firewall by sending packets to the counterpart
					ss.WriteFrame(trackID, StreamTypeRTP,
						[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
					ss.WriteFrame(trackID, StreamTypeRTCP,
						[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
				}

				return res, err
			}

			return res, liberrors.ErrServerTCPFramesEnable{}
		}

		ss.udpIP = nil
		ss.udpZone = ""
		ss.linkedConn = nil

		return res, err

	case base.Pause:
		err := ss.checkState(map[ServerSessionState]struct{}{
			ServerSessionStatePrePlay:   {},
			ServerSessionStatePlay:      {},
			ServerSessionStatePreRecord: {},
			ServerSessionStateRecord:    {},
		})
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, err
		}

		pathAndQuery, ok := req.URL.RTSPPath()
		if !ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerNoPath{}
		}

		// path can end with a slash due to Content-Base, remove it
		pathAndQuery = strings.TrimSuffix(pathAndQuery, "/")

		path, query := base.PathSplitQuery(pathAndQuery)

		res, err := ss.s.Handler.(ServerHandlerOnPause).OnPause(&ServerHandlerOnPauseCtx{
			Session: ss,
			Conn:    sc,
			Req:     req,
			Path:    path,
			Query:   query,
		})

		if res.StatusCode == base.StatusOK {
			switch ss.state {
			case ServerSessionStatePlay:
				ss.state = ServerSessionStatePrePlay
				ss.udpIP = nil
				ss.udpZone = ""
				ss.linkedConn = nil

				if *ss.setupProtocol == StreamProtocolUDP {
					ss.s.udpRTCPListener.removeClient(ss)
				} else {
					return res, liberrors.ErrServerTCPFramesDisable{}
				}

			case ServerSessionStateRecord:
				ss.state = ServerSessionStatePreRecord
				ss.udpIP = nil
				ss.udpZone = ""
				ss.linkedConn = nil

				if *ss.setupProtocol == StreamProtocolUDP {
					ss.s.udpRTPListener.removeClient(ss)
				} else {
					return res, liberrors.ErrServerTCPFramesDisable{}
				}
			}
		}

		return res, err

	case base.Teardown:
		ss.linkedConn = nil

		return &base.Response{
			StatusCode: base.StatusOK,
		}, liberrors.ErrServerSessionTeardown{}

	case base.GetParameter:
		if h, ok := sc.s.Handler.(ServerHandlerOnGetParameter); ok {
			pathAndQuery, ok := req.URL.RTSPPath()
			if !ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerNoPath{}
			}

			path, query := base.PathSplitQuery(pathAndQuery)

			return h.OnGetParameter(&ServerHandlerOnGetParameterCtx{
				Session: ss,
				Conn:    sc,
				Req:     req,
				Path:    path,
				Query:   query,
			})
		}

		// GET_PARAMETER is used like a ping when reading, and sometimes
		// also when publishing; reply with 200
		return &base.Response{
			StatusCode: base.StatusOK,
			Header: base.Header{
				"Content-Type": base.HeaderValue{"text/parameters"},
			},
			Body: []byte("\n"),
		}, nil
	}

	return &base.Response{
		StatusCode: base.StatusBadRequest,
	}, liberrors.ErrServerUnhandledRequest{Req: req}
}

// WriteFrame writes a frame.
func (ss *ServerSession) WriteFrame(trackID int, streamType StreamType, payload []byte) {
	if *ss.setupProtocol == StreamProtocolUDP {
		track := ss.setuppedTracks[trackID]

		if streamType == StreamTypeRTP {
			ss.s.udpRTPListener.write(payload, &net.UDPAddr{
				IP:   ss.udpIP,
				Zone: ss.udpZone,
				Port: track.udpRTPPort,
			})

		} else {
			ss.s.udpRTCPListener.write(payload, &net.UDPAddr{
				IP:   ss.udpIP,
				Zone: ss.udpZone,
				Port: track.udpRTCPPort,
			})
		}
	} else {
		ss.linkedConn.tcpFrameWriteBuffer.Push(&base.InterleavedFrame{
			TrackID:    trackID,
			StreamType: streamType,
			Payload:    payload,
		})
	}
}
