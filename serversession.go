package gortsplib

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
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

func setupGetTrackIDPathQuery(
	url *base.URL,
	thMode *headers.TransportMode,
	announcedTracks []ServerSessionAnnouncedTrack,
	setuppedPath *string,
	setuppedQuery *string,
	setuppedBaseURL *base.URL,
) (int, string, string, error) {
	pathAndQuery, ok := url.RTSPPathAndQuery()
	if !ok {
		return 0, "", "", liberrors.ErrServerInvalidPath{}
	}

	if thMode == nil || *thMode == headers.TransportModePlay {
		i := stringsReverseIndex(pathAndQuery, "/trackID=")

		// URL doesn't contain trackID - it's track zero
		if i < 0 {
			if !strings.HasSuffix(pathAndQuery, "/") {
				return 0, "", "", fmt.Errorf("path of a SETUP request must end with a slash (%v)", pathAndQuery)
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

		if setuppedPath != nil && (path != *setuppedPath || query != *setuppedQuery) {
			return 0, "", "", fmt.Errorf("can't setup tracks with different paths")
		}

		return trackID, path, query, nil
	}

	for trackID, track := range announcedTracks {
		u, _ := track.track.URL(setuppedBaseURL)
		if u.String() == url.String() {
			return trackID, *setuppedPath, *setuppedQuery, nil
		}
	}

	return 0, "", "", fmt.Errorf("invalid track path (%s)", pathAndQuery)
}

func setupGetTransport(th headers.Transport) (Transport, bool) {
	delivery := func() headers.TransportDelivery {
		if th.Delivery != nil {
			return *th.Delivery
		}
		return headers.TransportDeliveryUnicast
	}()

	switch th.Protocol {
	case headers.TransportProtocolUDP:
		if delivery == headers.TransportDeliveryUnicast {
			return TransportUDP, true
		}
		return TransportUDPMulticast, true

	default: // TCP
		if delivery != headers.TransportDeliveryUnicast {
			return 0, false
		}
		return TransportTCP, true
	}
}

// ServerSessionState is a state of a ServerSession.
type ServerSessionState int

// standard states.
const (
	ServerSessionStateInitial ServerSessionState = iota
	ServerSessionStatePreRead
	ServerSessionStateRead
	ServerSessionStatePrePublish
	ServerSessionStatePublish
)

// String implements fmt.Stringer.
func (s ServerSessionState) String() string {
	switch s {
	case ServerSessionStateInitial:
		return "initial"
	case ServerSessionStatePreRead:
		return "prePlay"
	case ServerSessionStateRead:
		return "play"
	case ServerSessionStatePrePublish:
		return "preRecord"
	case ServerSessionStatePublish:
		return "record"
	}
	return "unknown"
}

// ServerSessionSetuppedTrack is a setupped track of a ServerSession.
type ServerSessionSetuppedTrack struct {
	tcpChannel  int
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
	s        *Server
	secretID string // must not be shared, allows to take ownership of the session
	author   *ServerConn

	ctx                     context.Context
	ctxCancel               func()
	conns                   map[*ServerConn]struct{}
	state                   ServerSessionState
	setuppedTracks          map[int]ServerSessionSetuppedTrack
	setuppedTracksByChannel map[int]int // tcp
	setuppedTransport       *Transport
	setuppedBaseURL         *base.URL     // publish
	setuppedStream          *ServerStream // read
	setuppedPath            *string
	setuppedQuery           *string
	lastRequestTime         time.Time
	tcpConn                 *ServerConn                   // tcp
	announcedTracks         []ServerSessionAnnouncedTrack // publish
	udpLastFrameTime        *int64                        // publish, udp

	// in
	request    chan sessionRequestReq
	connRemove chan *ServerConn
}

func newServerSession(
	s *Server,
	secretID string,
	author *ServerConn,
) *ServerSession {
	ctx, ctxCancel := context.WithCancel(s.ctx)

	ss := &ServerSession{
		s:               s,
		secretID:        secretID,
		author:          author,
		ctx:             ctx,
		ctxCancel:       ctxCancel,
		conns:           make(map[*ServerConn]struct{}),
		lastRequestTime: time.Now(),
		request:         make(chan sessionRequestReq),
		connRemove:      make(chan *ServerConn),
	}

	s.wg.Add(1)
	go ss.run()

	return ss
}

// Close closes the ServerSession.
func (ss *ServerSession) Close() error {
	ss.ctxCancel()
	return nil
}

// State returns the state of the session.
func (ss *ServerSession) State() ServerSessionState {
	return ss.state
}

// SetuppedTracks returns the setupped tracks.
func (ss *ServerSession) SetuppedTracks() map[int]ServerSessionSetuppedTrack {
	return ss.setuppedTracks
}

// SetuppedTransport returns the transport of the setupped tracks.
func (ss *ServerSession) SetuppedTransport() *Transport {
	return ss.setuppedTransport
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
	return liberrors.ErrServerInvalidState{AllowedList: allowedList, State: ss.state}
}

func (ss *ServerSession) run() {
	defer ss.s.wg.Done()

	if h, ok := ss.s.Handler.(ServerHandlerOnSessionOpen); ok {
		h.OnSessionOpen(&ServerHandlerOnSessionOpenCtx{
			Session: ss,
			Conn:    ss.author,
		})
	}

	err := func() error {
		checkTimeoutTicker := time.NewTicker(serverSessionCheckStreamPeriod)
		defer checkTimeoutTicker.Stop()

		receiverReportTicker := time.NewTicker(ss.s.receiverReportPeriod)
		defer receiverReportTicker.Stop()

		for {
			select {
			case req := <-ss.request:
				ss.lastRequestTime = time.Now()

				if _, ok := ss.conns[req.sc]; !ok {
					ss.conns[req.sc] = struct{}{}
				}

				res, err := ss.handleRequest(req.sc, req.req)

				if res.StatusCode == base.StatusOK {
					if res.Header == nil {
						res.Header = make(base.Header)
					}
					res.Header["Session"] = base.HeaderValue{ss.secretID}
				}

				if _, ok := err.(liberrors.ErrServerSessionTeardown); ok {
					req.res <- sessionRequestRes{res: res, err: nil}
					return err
				}

				req.res <- sessionRequestRes{
					res: res,
					err: err,
					ss:  ss,
				}

			case sc := <-ss.connRemove:
				if _, ok := ss.conns[sc]; ok {
					delete(ss.conns, sc)

					select {
					case sc.sessionRemove <- ss:
					case <-sc.ctx.Done():
					}
				}

				// if session is not in state RECORD or PLAY, or transport is TCP
				if (ss.state != ServerSessionStatePublish &&
					ss.state != ServerSessionStateRead) ||
					*ss.setuppedTransport == TransportTCP {

					// close if there are no active connections
					if len(ss.conns) == 0 {
						return liberrors.ErrServerSessionTeardown{}
					}
				}

			case <-checkTimeoutTicker.C:
				switch {
				// in case of RECORD and UDP, timeout happens when no RTP or RTCP packets are being received
				case ss.state == ServerSessionStatePublish && (*ss.setuppedTransport == TransportUDP ||
					*ss.setuppedTransport == TransportUDPMulticast):
					now := time.Now()
					lft := atomic.LoadInt64(ss.udpLastFrameTime)
					if now.Sub(time.Unix(lft, 0)) >= ss.s.ReadTimeout {
						return liberrors.ErrServerNoUDPPacketsInAWhile{}
					}

					// in case of PLAY and UDP, timeout happens when no RTSP request arrives
				case ss.state == ServerSessionStateRead && (*ss.setuppedTransport == TransportUDP ||
					*ss.setuppedTransport == TransportUDPMulticast):
					now := time.Now()
					if now.Sub(ss.lastRequestTime) >= ss.s.closeSessionAfterNoRequestsFor {
						return liberrors.ErrServerNoRTSPRequestsInAWhile{}
					}

					// in case of TCP, there's no timeout until all associated connections are closed
				}

			case <-receiverReportTicker.C:
				if ss.state != ServerSessionStatePublish {
					continue
				}

				now := time.Now()
				for trackID, track := range ss.announcedTracks {
					r := track.rtcpReceiver.Report(now)
					ss.WriteFrame(trackID, StreamTypeRTCP, r)
				}

			case <-ss.ctx.Done():
				return liberrors.ErrServerTerminated{}
			}
		}
	}()

	ss.ctxCancel()

	switch ss.state {
	case ServerSessionStateRead:
		ss.setuppedStream.readerSetInactive(ss)

		if *ss.setuppedTransport == TransportUDP {
			ss.s.udpRTCPListener.removeClient(ss)
		}

	case ServerSessionStatePublish:
		if *ss.setuppedTransport == TransportUDP {
			ss.s.udpRTPListener.removeClient(ss)
			ss.s.udpRTCPListener.removeClient(ss)
		}
	}

	if ss.setuppedStream != nil {
		ss.setuppedStream.readerRemove(ss)
	}

	for sc := range ss.conns {
		if sc == ss.tcpConn {
			sc.Close()

			// make sure that OnFrame() is never called after OnSessionClose()
			<-sc.done
		}

		select {
		case sc.sessionRemove <- ss:
		case <-sc.ctx.Done():
		}
	}

	select {
	case ss.s.sessionClose <- ss:
	case <-ss.s.ctx.Done():
	}

	if h, ok := ss.s.Handler.(ServerHandlerOnSessionClose); ok {
		h.OnSessionClose(&ServerHandlerOnSessionCloseCtx{
			Session: ss,
			Error:   err,
		})
	}
}

func (ss *ServerSession) handleRequest(sc *ServerConn, req *base.Request) (*base.Response, error) {
	if ss.tcpConn != nil && sc != ss.tcpConn {
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

		pathAndQuery, ok := req.URL.RTSPPathAndQuery()
		if !ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerInvalidPath{}
		}

		path, query := base.PathSplitQuery(pathAndQuery)

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

		tracks, err := ReadTracks(req.Body)
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

		for _, track := range tracks {
			trackURL, err := track.URL(req.URL)
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("unable to generate track URL")
			}

			trackPath, ok := trackURL.RTSPPathAndQuery()
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
			Server:  ss.s,
			Session: ss,
			Conn:    sc,
			Req:     req,
			Path:    path,
			Query:   query,
			Tracks:  tracks,
		})

		if res.StatusCode != base.StatusOK {
			return res, err
		}

		ss.state = ServerSessionStatePrePublish
		ss.setuppedPath = &path
		ss.setuppedQuery = &query
		ss.setuppedBaseURL = req.URL

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
		return res, err

	case base.Setup:
		err := ss.checkState(map[ServerSessionState]struct{}{
			ServerSessionStateInitial:    {},
			ServerSessionStatePreRead:    {},
			ServerSessionStatePrePublish: {},
		})
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, err
		}

		var inTH headers.Transport
		err = inTH.Read(req.Header["Transport"])
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerTransportHeaderInvalid{Err: err}
		}

		trackID, path, query, err := setupGetTrackIDPathQuery(req.URL, inTH.Mode,
			ss.announcedTracks, ss.setuppedPath, ss.setuppedQuery, ss.setuppedBaseURL)
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

		transport, ok := setupGetTransport(inTH)
		if !ok {
			return &base.Response{
				StatusCode: base.StatusUnsupportedTransport,
			}, nil
		}

		switch transport {
		case TransportUDP:
			if inTH.ClientPorts == nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderNoClientPorts{}
			}

			if ss.s.udpRTPListener == nil {
				return &base.Response{
					StatusCode: base.StatusUnsupportedTransport,
				}, nil
			}

		case TransportUDPMulticast:
			if ss.s.MulticastIPRange == "" {
				return &base.Response{
					StatusCode: base.StatusUnsupportedTransport,
				}, nil
			}

		default: // TCP
			if inTH.InterleavedIDs == nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderNoInterleavedIDs{}
			}

			if (inTH.InterleavedIDs[0]%2) != 0 ||
				(inTH.InterleavedIDs[0]+1) != inTH.InterleavedIDs[1] {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderInvalidInterleavedIDs{}
			}

			if _, ok := ss.setuppedTracksByChannel[inTH.InterleavedIDs[0]]; ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderInterleavedIDsAlreadyUsed{}
			}
		}

		if ss.setuppedTransport != nil && *ss.setuppedTransport != transport {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerTracksDifferentProtocols{}
		}

		switch ss.state {
		case ServerSessionStateInitial, ServerSessionStatePreRead: // play
			if inTH.Mode != nil && *inTH.Mode != headers.TransportModePlay {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderInvalidMode{Mode: inTH.Mode}
			}

		default: // record
			if transport == TransportUDPMulticast {
				return &base.Response{
					StatusCode: base.StatusUnsupportedTransport,
				}, nil
			}

			if inTH.Mode == nil || *inTH.Mode != headers.TransportModeRecord {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderInvalidMode{Mode: inTH.Mode}
			}
		}

		res, stream, err := ss.s.Handler.(ServerHandlerOnSetup).OnSetup(&ServerHandlerOnSetupCtx{
			Server:    ss.s,
			Session:   ss,
			Conn:      sc,
			Req:       req,
			Path:      path,
			Query:     query,
			TrackID:   trackID,
			Transport: transport,
		})

		// workaround to prevent a bug in rtspclientsink
		// that makes impossible for the client to receive the response
		// and send frames.
		// this was causing problems during unit tests.
		if ua, ok := req.Header["User-Agent"]; ok && len(ua) == 1 &&
			strings.HasPrefix(ua[0], "GStreamer") {
			select {
			case <-time.After(1 * time.Second):
			case <-ss.ctx.Done():
			}
		}

		if res.StatusCode != base.StatusOK {
			return res, err
		}

		if ss.state == ServerSessionStateInitial {
			err := stream.readerAdd(ss,
				transport,
				inTH.ClientPorts,
			)
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, err
			}

			ss.state = ServerSessionStatePreRead
			ss.setuppedPath = &path
			ss.setuppedQuery = &query
			ss.setuppedStream = stream
		}

		th := headers.Transport{}

		if ss.state == ServerSessionStatePreRead {
			ssrc := stream.ssrc(trackID)
			if ssrc != 0 {
				th.SSRC = &ssrc
			}
		}

		ss.setuppedTransport = &transport

		if res.Header == nil {
			res.Header = make(base.Header)
		}

		sst := ServerSessionSetuppedTrack{}

		switch transport {
		case TransportUDP:
			sst.udpRTPPort = inTH.ClientPorts[0]
			sst.udpRTCPPort = inTH.ClientPorts[1]

			th.Protocol = headers.TransportProtocolUDP
			de := headers.TransportDeliveryUnicast
			th.Delivery = &de
			th.ClientPorts = inTH.ClientPorts
			th.ServerPorts = &[2]int{sc.s.udpRTPListener.port(), sc.s.udpRTCPListener.port()}

		case TransportUDPMulticast:
			th.Protocol = headers.TransportProtocolUDP
			de := headers.TransportDeliveryMulticast
			th.Delivery = &de
			v := uint(127)
			th.TTL = &v
			d := stream.multicastListeners[trackID].rtpListener.ip()
			th.Destination = &d
			th.Ports = &[2]int{
				stream.multicastListeners[trackID].rtpListener.port(),
				stream.multicastListeners[trackID].rtcpListener.port(),
			}

		default: // TCP
			sst.tcpChannel = inTH.InterleavedIDs[0]

			if ss.setuppedTracksByChannel == nil {
				ss.setuppedTracksByChannel = make(map[int]int)
			}

			ss.setuppedTracksByChannel[inTH.InterleavedIDs[0]] = trackID

			th.Protocol = headers.TransportProtocolTCP
			de := headers.TransportDeliveryUnicast
			th.Delivery = &de
			th.InterleavedIDs = inTH.InterleavedIDs
		}

		if ss.setuppedTracks == nil {
			ss.setuppedTracks = make(map[int]ServerSessionSetuppedTrack)
		}

		ss.setuppedTracks[trackID] = sst

		res.Header["Transport"] = th.Write()

		return res, err

	case base.Play:
		// play can be sent twice, allow calling it even if we're already playing
		err := ss.checkState(map[ServerSessionState]struct{}{
			ServerSessionStatePreRead: {},
			ServerSessionStateRead:    {},
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

		pathAndQuery, ok := req.URL.RTSPPathAndQuery()
		if !ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerInvalidPath{}
		}

		// path can end with a slash due to Content-Base, remove it
		pathAndQuery = strings.TrimSuffix(pathAndQuery, "/")

		path, query := base.PathSplitQuery(pathAndQuery)

		if ss.State() == ServerSessionStatePreRead &&
			path != *ss.setuppedPath {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerPathHasChanged{Prev: *ss.setuppedPath, Cur: path}
		}

		res, err := sc.s.Handler.(ServerHandlerOnPlay).OnPlay(&ServerHandlerOnPlayCtx{
			Session: ss,
			Conn:    sc,
			Req:     req,
			Path:    path,
			Query:   query,
		})

		if res.StatusCode != base.StatusOK {
			return res, err
		}

		if ss.state == ServerSessionStateRead {
			return res, err
		}

		ss.state = ServerSessionStateRead

		if *ss.setuppedTransport == TransportTCP {
			ss.tcpConn = sc
		}

		// add RTP-Info
		var trackIDs []int
		for trackID := range ss.setuppedTracks {
			trackIDs = append(trackIDs, trackID)
		}
		sort.Slice(trackIDs, func(a, b int) bool {
			return trackIDs[a] < trackIDs[b]
		})
		var ri headers.RTPInfo
		for _, trackID := range trackIDs {
			ts := ss.setuppedStream.timestamp(trackID)
			if ts == 0 {
				continue
			}

			u := &base.URL{
				Scheme: req.URL.Scheme,
				User:   req.URL.User,
				Host:   req.URL.Host,
				Path:   "/" + *ss.setuppedPath + "/trackID=" + strconv.FormatInt(int64(trackID), 10),
			}

			lsn := ss.setuppedStream.lastSequenceNumber(trackID)

			ri = append(ri, &headers.RTPInfoEntry{
				URL:            u.String(),
				SequenceNumber: &lsn,
				Timestamp:      &ts,
			})
		}
		if len(ri) > 0 {
			if res.Header == nil {
				res.Header = make(base.Header)
			}
			res.Header["RTP-Info"] = ri.Write()
		}

		ss.setuppedStream.readerSetActive(ss)

		switch *ss.setuppedTransport {
		case TransportUDP:
			for trackID, track := range ss.setuppedTracks {
				// readers can send RTCP packets
				sc.s.udpRTCPListener.addClient(ss.author.ip(), track.udpRTCPPort, ss, trackID, false)

				// open the firewall by sending packets to the counterpart
				ss.WriteFrame(trackID, StreamTypeRTCP,
					[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
			}

			return res, err

		case TransportUDPMulticast:

		default: // TCP
			err = liberrors.ErrServerTCPFramesEnable{}
		}

		return res, err

	case base.Record:
		err := ss.checkState(map[ServerSessionState]struct{}{
			ServerSessionStatePrePublish: {},
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

		pathAndQuery, ok := req.URL.RTSPPathAndQuery()
		if !ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerInvalidPath{}
		}

		// path can end with a slash due to Content-Base, remove it
		pathAndQuery = strings.TrimSuffix(pathAndQuery, "/")

		path, query := base.PathSplitQuery(pathAndQuery)

		// allow to use WriteFrame() before response
		if *ss.setuppedTransport == TransportTCP {
			ss.tcpConn = sc
		}

		if path != *ss.setuppedPath {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerPathHasChanged{Prev: *ss.setuppedPath, Cur: path}
		}

		res, err := ss.s.Handler.(ServerHandlerOnRecord).OnRecord(&ServerHandlerOnRecordCtx{
			Session: ss,
			Conn:    sc,
			Req:     req,
			Path:    path,
			Query:   query,
		})

		if res.StatusCode != base.StatusOK {
			ss.tcpConn = nil
			return res, err
		}

		ss.state = ServerSessionStatePublish

		switch *ss.setuppedTransport {
		case TransportUDP:
			for trackID, track := range ss.setuppedTracks {
				ss.s.udpRTPListener.addClient(ss.author.ip(), track.udpRTPPort, ss, trackID, true)
				ss.s.udpRTCPListener.addClient(ss.author.ip(), track.udpRTCPPort, ss, trackID, true)

				// open the firewall by sending packets to the counterpart
				ss.WriteFrame(trackID, StreamTypeRTP,
					[]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
				ss.WriteFrame(trackID, StreamTypeRTCP,
					[]byte{0x80, 0xc9, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
			}

		case TransportUDPMulticast:

		default: // TCP
			err = liberrors.ErrServerTCPFramesEnable{}
		}

		return res, err

	case base.Pause:
		err := ss.checkState(map[ServerSessionState]struct{}{
			ServerSessionStatePreRead:    {},
			ServerSessionStateRead:       {},
			ServerSessionStatePrePublish: {},
			ServerSessionStatePublish:    {},
		})
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, err
		}

		pathAndQuery, ok := req.URL.RTSPPathAndQuery()
		if !ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerInvalidPath{}
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

		if res.StatusCode != base.StatusOK {
			return res, err
		}

		switch ss.state {
		case ServerSessionStateRead:
			ss.setuppedStream.readerSetInactive(ss)

			ss.state = ServerSessionStatePreRead
			ss.tcpConn = nil

			switch *ss.setuppedTransport {
			case TransportUDP:
				ss.s.udpRTCPListener.removeClient(ss)

			case TransportUDPMulticast:

			default: // TCP
				err = liberrors.ErrServerTCPFramesDisable{}
			}

		case ServerSessionStatePublish:
			ss.state = ServerSessionStatePrePublish
			ss.tcpConn = nil

			switch *ss.setuppedTransport {
			case TransportUDP:
				ss.s.udpRTPListener.removeClient(ss)
				ss.s.udpRTCPListener.removeClient(ss)

			case TransportUDPMulticast:

			default: // TCP
				err = liberrors.ErrServerTCPFramesDisable{}
			}
		}

		return res, err

	case base.Teardown:
		return &base.Response{
			StatusCode: base.StatusOK,
		}, liberrors.ErrServerSessionTeardown{Author: sc.NetConn().RemoteAddr()}

	case base.GetParameter:
		if h, ok := sc.s.Handler.(ServerHandlerOnGetParameter); ok {
			pathAndQuery, ok := req.URL.RTSPPathAndQuery()
			if !ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerInvalidPath{}
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

// WriteFrame writes a frame to the session.
func (ss *ServerSession) WriteFrame(trackID int, streamType StreamType, payload []byte) {
	if _, ok := ss.setuppedTracks[trackID]; !ok {
		return
	}

	switch *ss.setuppedTransport {
	case TransportUDP:
		track := ss.setuppedTracks[trackID]

		if streamType == StreamTypeRTP {
			ss.s.udpRTPListener.write(payload, &net.UDPAddr{
				IP:   ss.author.ip(),
				Zone: ss.author.zone(),
				Port: track.udpRTPPort,
			})
		} else {
			ss.s.udpRTCPListener.write(payload, &net.UDPAddr{
				IP:   ss.author.ip(),
				Zone: ss.author.zone(),
				Port: track.udpRTCPPort,
			})
		}

	case TransportTCP:
		channel := ss.setuppedTracks[trackID].tcpChannel
		if streamType == base.StreamTypeRTCP {
			channel++
		}

		ss.tcpConn.tcpFrameWriteBuffer.Push(&base.InterleavedFrame{
			Channel: channel,
			Payload: payload,
		})
	}
}
