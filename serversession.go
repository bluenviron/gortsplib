package gortsplib

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/base"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/headers"
	"github.com/aler9/gortsplib/v2/pkg/liberrors"
	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/aler9/gortsplib/v2/pkg/ringbuffer"
	"github.com/aler9/gortsplib/v2/pkg/sdp"
	"github.com/aler9/gortsplib/v2/pkg/url"
)

func stringsReverseIndex(s, substr string) int {
	for i := len(s) - 1 - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func serverParseURLForPlay(u *url.URL) (string, string, int, error) {
	pathAndQuery, ok := u.RTSPPathAndQuery()
	if !ok {
		return "", "", -1, liberrors.ErrServerInvalidPath{}
	}

	i := stringsReverseIndex(pathAndQuery, "/mediaID=")
	if i < 0 {
		if !strings.HasSuffix(pathAndQuery, "/") {
			return "", "", -1, fmt.Errorf("path of a SETUP request must end with a slash. " +
				"This typically happens when VLC fails a request, and then switches to an " +
				"unsupported RTSP dialect")
		}

		path, query := url.PathSplitQuery(pathAndQuery[:len(pathAndQuery)-1])
		return path, query, 0, nil
	}

	var t string
	pathAndQuery, t = pathAndQuery[:i], pathAndQuery[i+len("/mediaID="):]
	path, query := url.PathSplitQuery(pathAndQuery)
	tmp, _ := strconv.ParseInt(t, 10, 64)
	return path, query, int(tmp), nil
}

func findMediaByURL(medias media.Medias, baseURL *url.URL, u *url.URL) (*media.Media, bool) {
	for _, media := range medias {
		u1, err := media.URL(baseURL)
		if err == nil && u1.String() == u.String() {
			return media, true
		}
	}

	return nil, false
}

func findMediaByID(medias media.Medias, id int) (*media.Media, bool) {
	if len(medias) <= id {
		return nil, false
	}
	return medias[id], true
}

func findFirstSupportedTransportHeader(s *Server, tsh headers.Transports) *headers.Transport {
	// Per RFC2326 section 12.39, client specifies transports in order of preference.
	// Filter out the ones we don't support and then pick first supported transport.
	for _, tr := range tsh {
		isMulticast := tr.Delivery != nil && *tr.Delivery == headers.TransportDeliveryMulticast
		if tr.Protocol == headers.TransportProtocolUDP &&
			((!isMulticast && s.udpRTPListener == nil) ||
				(isMulticast && s.MulticastIPRange == "")) {
			continue
		}
		return &tr
	}
	return nil
}

func findAndValidateTransport(inTH *headers.Transport,
	tcpMediasByChannel map[int]*serverSessionMedia,
) (Transport, error) {
	if inTH.Protocol == headers.TransportProtocolUDP {
		if inTH.Delivery != nil && *inTH.Delivery == headers.TransportDeliveryMulticast {
			return TransportUDPMulticast, nil
		}

		if inTH.ClientPorts == nil {
			return 0, liberrors.ErrServerTransportHeaderNoClientPorts{}
		}
		return TransportUDP, nil
	}

	if inTH.InterleavedIDs == nil {
		return 0, liberrors.ErrServerTransportHeaderNoInterleavedIDs{}
	}

	if (inTH.InterleavedIDs[0]%2) != 0 ||
		(inTH.InterleavedIDs[0]+1) != inTH.InterleavedIDs[1] {
		return 0, liberrors.ErrServerTransportHeaderInvalidInterleavedIDs{}
	}

	if _, ok := tcpMediasByChannel[inTH.InterleavedIDs[0]]; ok {
		return 0, liberrors.ErrServerTransportHeaderInterleavedIDsAlreadyUsed{}
	}

	return TransportTCP, nil
}

// ServerSessionState is a state of a ServerSession.
type ServerSessionState int

// states.
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

// ServerSession is a server-side RTSP session.
type ServerSession struct {
	s        *Server
	secretID string // must not be shared, allows to take ownership of the session
	author   *ServerConn

	ctx                   context.Context
	ctxCancel             func()
	bytesReceived         *uint64
	bytesSent             *uint64
	userData              interface{}
	conns                 map[*ServerConn]struct{}
	state                 ServerSessionState
	setuppedMedias        map[*media.Media]*serverSessionMedia
	setuppedMediasOrdered []*serverSessionMedia
	tcpMediasByChannel    map[int]*serverSessionMedia
	setuppedTransport     *Transport
	setuppedStream        *ServerStream // read
	setuppedPath          *string
	setuppedQuery         string
	lastRequestTime       time.Time
	tcpConn               *ServerConn
	announcedMedias       media.Medias // publish
	udpLastPacketTime     *int64       // publish
	udpCheckStreamTimer   *time.Timer
	writer                serverWriter
	rtpPacketBuffer       *rtpPacketMultiBuffer

	// in
	request     chan sessionRequestReq
	connRemove  chan *ServerConn
	startWriter chan struct{}
}

func newServerSession(
	s *Server,
	secretID string,
	author *ServerConn,
) *ServerSession {
	ctx, ctxCancel := context.WithCancel(s.ctx)

	ss := &ServerSession{
		s:                   s,
		secretID:            secretID,
		author:              author,
		ctx:                 ctx,
		ctxCancel:           ctxCancel,
		bytesReceived:       new(uint64),
		bytesSent:           new(uint64),
		conns:               make(map[*ServerConn]struct{}),
		lastRequestTime:     time.Now(),
		udpCheckStreamTimer: emptyTimer(),
		request:             make(chan sessionRequestReq),
		connRemove:          make(chan *ServerConn),
		startWriter:         make(chan struct{}),
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

// BytesReceived returns the number of read bytes.
func (ss *ServerSession) BytesReceived() uint64 {
	return atomic.LoadUint64(ss.bytesReceived)
}

// BytesSent returns the number of written bytes.
func (ss *ServerSession) BytesSent() uint64 {
	return atomic.LoadUint64(ss.bytesSent)
}

// State returns the state of the session.
func (ss *ServerSession) State() ServerSessionState {
	return ss.state
}

// SetuppedTransport returns the transport negotiated during SETUP.
func (ss *ServerSession) SetuppedTransport() *Transport {
	return ss.setuppedTransport
}

// AnnouncedMedias returns the announced media.
func (ss *ServerSession) AnnouncedMedias() media.Medias {
	return ss.announcedMedias
}

// SetUserData sets some user data associated to the session.
func (ss *ServerSession) SetUserData(v interface{}) {
	ss.userData = v
}

// UserData returns some user data associated to the session.
func (ss *ServerSession) UserData() interface{} {
	return ss.userData
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

	err := ss.runInner()

	ss.ctxCancel()

	if ss.setuppedStream != nil {
		ss.setuppedStream.readerSetInactive(ss)
		ss.setuppedStream.readerRemove(ss)
	}

	for _, sm := range ss.setuppedMedias {
		sm.stop()
	}

	ss.writer.stop()

	// close all associated connections, both UDP and TCP
	// except for the ones that called TEARDOWN
	// (that are detached from the session just after the request)
	for sc := range ss.conns {
		sc.Close()

		// make sure that OnFrame() is never called after OnSessionClose()
		<-sc.done

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

func (ss *ServerSession) runInner() error {
	for {
		select {
		case req := <-ss.request:
			ss.lastRequestTime = time.Now()

			if _, ok := ss.conns[req.sc]; !ok {
				ss.conns[req.sc] = struct{}{}
			}

			res, err := ss.handleRequest(req.sc, req.req)

			returnedSession := ss

			if err == nil || err == errSwitchReadFunc {
				// ANNOUNCE responses don't contain the session header.
				if req.req.Method != base.Announce &&
					req.req.Method != base.Teardown {
					if res.Header == nil {
						res.Header = make(base.Header)
					}

					res.Header["Session"] = headers.Session{
						Session: ss.secretID,
						Timeout: func() *uint {
							// timeout controls the sending of RTCP keepalives.
							// these are needed only when the client is playing
							// and transport is UDP or UDP-multicast.
							if (ss.state == ServerSessionStatePrePlay ||
								ss.state == ServerSessionStatePlay) &&
								(*ss.setuppedTransport == TransportUDP ||
									*ss.setuppedTransport == TransportUDPMulticast) {
								v := uint(ss.s.sessionTimeout / time.Second)
								return &v
							}
							return nil
						}(),
					}.Marshal()
				}

				// after a TEARDOWN, session must be unpaired with the connection
				if req.req.Method == base.Teardown {
					delete(ss.conns, req.sc)
					returnedSession = nil
				}
			}

			savedMethod := req.req.Method

			req.res <- sessionRequestRes{
				res: res,
				err: err,
				ss:  returnedSession,
			}

			if (err == nil || err == errSwitchReadFunc) && savedMethod == base.Teardown {
				return liberrors.ErrServerSessionTeardown{Author: req.sc.NetConn().RemoteAddr()}
			}

		case sc := <-ss.connRemove:
			delete(ss.conns, sc)

			// if session is not in state RECORD or PLAY, or transport is TCP,
			// and there are no associated connections,
			// close the session.
			if ((ss.state != ServerSessionStateRecord &&
				ss.state != ServerSessionStatePlay) ||
				*ss.setuppedTransport == TransportTCP) &&
				len(ss.conns) == 0 {
				return liberrors.ErrServerSessionNotInUse{}
			}

		case <-ss.startWriter:
			if (ss.state == ServerSessionStateRecord ||
				ss.state == ServerSessionStatePlay) &&
				*ss.setuppedTransport == TransportTCP {
				ss.writer.start()
			}

		case <-ss.udpCheckStreamTimer.C:
			now := time.Now()

			// in case of RECORD, timeout happens when no RTP or RTCP packets are being received
			if ss.state == ServerSessionStateRecord {
				lft := atomic.LoadInt64(ss.udpLastPacketTime)
				if now.Sub(time.Unix(lft, 0)) >= ss.s.ReadTimeout {
					return liberrors.ErrServerNoUDPPacketsInAWhile{}
				}

				// in case of PLAY, timeout happens when no RTSP keepalives are being received
			} else if now.Sub(ss.lastRequestTime) >= ss.s.sessionTimeout {
				return liberrors.ErrServerNoRTSPRequestsInAWhile{}
			}

			ss.udpCheckStreamTimer = time.NewTimer(ss.s.checkStreamPeriod)

		case <-ss.ctx.Done():
			return liberrors.ErrServerTerminated{}
		}
	}
}

func (ss *ServerSession) handleRequest(sc *ServerConn, req *base.Request) (*base.Response, error) {
	if ss.tcpConn != nil && sc != ss.tcpConn {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, liberrors.ErrServerSessionLinkedToOtherConn{}
	}

	var path string
	var query string
	switch req.Method {
	case base.Announce, base.Play, base.Record, base.Pause, base.GetParameter, base.SetParameter:
		pathAndQuery, ok := req.URL.RTSPPathAndQuery()
		if !ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerInvalidPath{}
		}

		if req.Method != base.Announce {
			// path can end with a slash due to Content-Base, remove it
			pathAndQuery = strings.TrimSuffix(pathAndQuery, "/")
		}

		path, query = url.PathSplitQuery(pathAndQuery)
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

		var sd sdp.SessionDescription
		err = sd.Unmarshal(req.Body)
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerSDPInvalid{Err: err}
		}

		var medias media.Medias
		err = medias.Unmarshal(sd.MediaDescriptions)
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerSDPInvalid{Err: err}
		}

		for _, medi := range medias {
			mediURL, err := medi.URL(req.URL)
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("unable to generate media URL")
			}

			mediPath, ok := mediURL.RTSPPathAndQuery()
			if !ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, fmt.Errorf("invalid media URL (%v)", mediURL)
			}

			if !strings.HasPrefix(mediPath, path) {
				return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, fmt.Errorf("invalid media path: must begin with '%s', but is '%s'",
						path, mediPath)
			}
		}

		res, err := ss.s.Handler.(ServerHandlerOnAnnounce).OnAnnounce(&ServerHandlerOnAnnounceCtx{
			Server:  ss.s,
			Session: ss,
			Conn:    sc,
			Request: req,
			Path:    path,
			Query:   query,
			Medias:  medias,
		})

		if res.StatusCode != base.StatusOK {
			return res, err
		}

		ss.state = ServerSessionStatePreRecord
		ss.setuppedPath = &path
		ss.setuppedQuery = query
		ss.announcedMedias = medias

		v := time.Now().Unix()
		ss.udpLastPacketTime = &v
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

		var inTSH headers.Transports
		err = inTSH.Unmarshal(req.Header["Transport"])
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerTransportHeaderInvalid{Err: err}
		}

		inTH := findFirstSupportedTransportHeader(ss.s, inTSH)
		if inTH == nil {
			return &base.Response{
				StatusCode: base.StatusUnsupportedTransport,
			}, nil
		}

		var path string
		var query string
		var mediaID int
		switch ss.state {
		case ServerSessionStateInitial, ServerSessionStatePrePlay: // play
			var err error
			path, query, mediaID, err = serverParseURLForPlay(req.URL)
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, err
			}

			if ss.setuppedPath != nil && path != *ss.setuppedPath {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerMediasDifferentPaths{}
			}

		default: // record
			path = *ss.setuppedPath
			query = ss.setuppedQuery
		}

		transport, err := findAndValidateTransport(inTH, ss.tcpMediasByChannel)
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, err
		}

		if ss.setuppedTransport != nil && *ss.setuppedTransport != transport {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerMediasDifferentProtocols{}
		}

		switch ss.state {
		case ServerSessionStateInitial, ServerSessionStatePrePlay: // play
			if inTH.Mode != nil && *inTH.Mode != headers.TransportModePlay {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTransportHeaderInvalidMode{Mode: *inTH.Mode}
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
				}, liberrors.ErrServerTransportHeaderInvalidMode{Mode: *inTH.Mode}
			}
		}

		res, stream, err := ss.s.Handler.(ServerHandlerOnSetup).OnSetup(&ServerHandlerOnSetupCtx{
			Server:    ss.s,
			Session:   ss,
			Conn:      sc,
			Request:   req,
			Path:      path,
			Query:     query,
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

		var medi *media.Media
		var ok bool
		switch ss.state {
		case ServerSessionStateInitial, ServerSessionStatePrePlay: // play
			medi, ok = findMediaByID(stream.medias, mediaID)
		default: // record
			medi, ok = findMediaByURL(ss.announcedMedias, &url.URL{
				Scheme:   req.URL.Scheme,
				Host:     req.URL.Host,
				Path:     path,
				RawQuery: query,
			}, req.URL)
		}

		if !ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, fmt.Errorf("media not found")
		}

		if _, ok := ss.setuppedMedias[medi]; ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerMediaAlreadySetup{}
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

			ss.state = ServerSessionStatePrePlay
			ss.setuppedPath = &path
			ss.setuppedStream = stream
		}

		th := headers.Transport{}

		if ss.state == ServerSessionStatePrePlay {
			ssrc, ok := stream.lastSSRC(medi)
			if ok {
				th.SSRC = &ssrc
			}
		}

		ss.setuppedTransport = &transport

		if res.Header == nil {
			res.Header = make(base.Header)
		}

		sm := newServerSessionMedia(ss, medi)

		if ss.state == ServerSessionStatePreRecord {
			sm.formats = make(map[uint8]*serverSessionFormat)
			for _, trak := range sm.media.Formats {
				sm.formats[trak.PayloadType()] = newServerSessionFormat(sm, trak)
			}
		}

		switch transport {
		case TransportUDP:
			sm.udpRTPReadPort = inTH.ClientPorts[0]
			sm.udpRTCPReadPort = inTH.ClientPorts[1]

			sm.udpRTPWriteAddr = &net.UDPAddr{
				IP:   ss.author.ip(),
				Zone: ss.author.zone(),
				Port: sm.udpRTPReadPort,
			}

			sm.udpRTCPWriteAddr = &net.UDPAddr{
				IP:   ss.author.ip(),
				Zone: ss.author.zone(),
				Port: sm.udpRTCPReadPort,
			}

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
			d := stream.streamMedias[medi].multicastHandler.ip()
			th.Destination = &d
			th.Ports = &[2]int{ss.s.MulticastRTPPort, ss.s.MulticastRTCPPort}

		default: // TCP
			sm.tcpChannel = inTH.InterleavedIDs[0]

			if ss.tcpMediasByChannel == nil {
				ss.tcpMediasByChannel = make(map[int]*serverSessionMedia)
			}

			ss.tcpMediasByChannel[inTH.InterleavedIDs[0]] = sm

			th.Protocol = headers.TransportProtocolTCP
			de := headers.TransportDeliveryUnicast
			th.Delivery = &de
			th.InterleavedIDs = inTH.InterleavedIDs
		}

		if ss.setuppedMedias == nil {
			ss.setuppedMedias = make(map[*media.Media]*serverSessionMedia)
		}
		ss.setuppedMedias[medi] = sm
		ss.setuppedMediasOrdered = append(ss.setuppedMediasOrdered, sm)

		res.Header["Transport"] = th.Marshal()

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

		if ss.State() == ServerSessionStatePrePlay &&
			path != *ss.setuppedPath {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerPathHasChanged{Prev: *ss.setuppedPath, Cur: path}
		}

		// allocate writeBuffer before calling OnPlay().
		// in this way it's possible to call ServerSession.WritePacket*()
		// inside the callback.
		if ss.state != ServerSessionStatePlay &&
			*ss.setuppedTransport != TransportUDPMulticast {
			ss.writer.buffer, _ = ringbuffer.New(uint64(ss.s.WriteBufferCount))
		}

		res, err := sc.s.Handler.(ServerHandlerOnPlay).OnPlay(&ServerHandlerOnPlayCtx{
			Session: ss,
			Conn:    sc,
			Request: req,
			Path:    path,
			Query:   query,
		})

		if res.StatusCode != base.StatusOK {
			if ss.state != ServerSessionStatePlay {
				ss.writer.buffer = nil
			}
			return res, err
		}

		if ss.state == ServerSessionStatePlay {
			return res, err
		}

		ss.state = ServerSessionStatePlay

		for _, sm := range ss.setuppedMedias {
			sm.start()
		}

		ss.setuppedStream.readerSetActive(ss)

		switch *ss.setuppedTransport {
		case TransportUDP:
			ss.udpCheckStreamTimer = time.NewTimer(ss.s.checkStreamPeriod)
			ss.writer.start()

		case TransportUDPMulticast:
			ss.udpCheckStreamTimer = time.NewTimer(ss.s.checkStreamPeriod)

		default: // TCP
			ss.tcpConn = sc
			ss.tcpConn.readFunc = ss.tcpConn.readFuncTCP
			err = errSwitchReadFunc
			// writer.start() is called by ServerConn after the response has been sent
		}

		var ri headers.RTPInfo
		now := time.Now()

		for i, sm := range ss.setuppedMediasOrdered {
			entry := ss.setuppedStream.rtpInfoEntry(sm.media, now)
			if entry != nil {
				entry.URL = (&url.URL{
					Scheme: req.URL.Scheme,
					Host:   req.URL.Host,
					Path:   "/" + *ss.setuppedPath + "/mediaID=" + strconv.FormatInt(int64(i), 10),
				}).String()

				ri = append(ri, entry)
			}
		}
		if len(ri) > 0 {
			if res.Header == nil {
				res.Header = make(base.Header)
			}
			res.Header["RTP-Info"] = ri.Marshal()
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

		if len(ss.setuppedMedias) != len(ss.announcedMedias) {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerNotAllAnnouncedMediasSetup{}
		}

		if path != *ss.setuppedPath {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerPathHasChanged{Prev: *ss.setuppedPath, Cur: path}
		}

		// allocate writeBuffer before calling OnRecord().
		// in this way it's possible to call ServerSession.WritePacket*()
		// inside the callback.
		// when recording, writeBuffer is only used to send RTCP receiver reports,
		// that are much smaller than RTP packets and are sent at a fixed interval.
		// decrease RAM consumption by allocating less buffers.
		ss.writer.buffer, _ = ringbuffer.New(uint64(8))

		res, err := ss.s.Handler.(ServerHandlerOnRecord).OnRecord(&ServerHandlerOnRecordCtx{
			Session: ss,
			Conn:    sc,
			Request: req,
			Path:    path,
			Query:   query,
		})

		if res.StatusCode != base.StatusOK {
			ss.writer.buffer = nil
			return res, err
		}

		ss.state = ServerSessionStateRecord

		ss.rtpPacketBuffer = newRTPPacketMultiBuffer(uint64(ss.s.ReadBufferCount))

		for _, sm := range ss.setuppedMedias {
			sm.start()
		}

		switch *ss.setuppedTransport {
		case TransportUDP:
			ss.udpCheckStreamTimer = time.NewTimer(ss.s.checkStreamPeriod)
			ss.writer.start()

		default: // TCP
			ss.tcpConn = sc
			ss.tcpConn.readFunc = ss.tcpConn.readFuncTCP
			err = errSwitchReadFunc
			// runWriter() is called by conn after sending the response
		}

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

		res, err := ss.s.Handler.(ServerHandlerOnPause).OnPause(&ServerHandlerOnPauseCtx{
			Session: ss,
			Conn:    sc,
			Request: req,
			Path:    path,
			Query:   query,
		})

		if res.StatusCode != base.StatusOK {
			return res, err
		}

		ss.writer.stop()

		if ss.setuppedStream != nil {
			ss.setuppedStream.readerSetInactive(ss)
		}

		for _, sm := range ss.setuppedMedias {
			sm.stop()
		}

		switch ss.state {
		case ServerSessionStatePlay:
			ss.state = ServerSessionStatePrePlay

			switch *ss.setuppedTransport {
			case TransportUDP:
				ss.udpCheckStreamTimer = emptyTimer()

			case TransportUDPMulticast:
				ss.udpCheckStreamTimer = emptyTimer()

			default: // TCP
				ss.tcpConn.readFunc = ss.tcpConn.readFuncStandard
				err = errSwitchReadFunc
				ss.tcpConn = nil
			}

		case ServerSessionStateRecord:
			switch *ss.setuppedTransport {
			case TransportUDP:
				ss.udpCheckStreamTimer = emptyTimer()

			default: // TCP
				ss.tcpConn.readFunc = ss.tcpConn.readFuncStandard
				err = errSwitchReadFunc
				ss.tcpConn = nil
			}

			ss.state = ServerSessionStatePreRecord
		}

		return res, err

	case base.Teardown:
		var err error
		if (ss.state == ServerSessionStatePlay || ss.state == ServerSessionStateRecord) &&
			*ss.setuppedTransport == TransportTCP {
			ss.tcpConn.readFunc = ss.tcpConn.readFuncStandard
			err = errSwitchReadFunc
		}

		return &base.Response{
			StatusCode: base.StatusOK,
		}, err

	case base.GetParameter:
		if h, ok := sc.s.Handler.(ServerHandlerOnGetParameter); ok {
			return h.OnGetParameter(&ServerHandlerOnGetParameterCtx{
				Session: ss,
				Conn:    sc,
				Request: req,
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
			Body: []byte{},
		}, nil

	case base.SetParameter:
		if h, ok := sc.s.Handler.(ServerHandlerOnSetParameter); ok {
			return h.OnSetParameter(&ServerHandlerOnSetParameterCtx{
				Session: ss,
				Conn:    sc,
				Request: req,
				Path:    path,
				Query:   query,
			})
		}
	}

	return &base.Response{
		StatusCode: base.StatusNotImplemented,
	}, nil
}

// OnPacketRTPAny sets the callback that is called when a RTP packet is read from any setupped media.
func (ss *ServerSession) OnPacketRTPAny(cb func(*media.Media, format.Format, *rtp.Packet)) {
	for _, sm := range ss.setuppedMedias {
		cmedia := sm.media
		for _, trak := range sm.media.Formats {
			ss.OnPacketRTP(sm.media, trak, func(pkt *rtp.Packet) {
				cb(cmedia, trak, pkt)
			})
		}
	}
}

// OnPacketRTCPAny sets the callback that is called when a RTCP packet is read from any setupped media.
func (ss *ServerSession) OnPacketRTCPAny(cb func(*media.Media, rtcp.Packet)) {
	for _, sm := range ss.setuppedMedias {
		cmedia := sm.media
		ss.OnPacketRTCP(sm.media, func(pkt rtcp.Packet) {
			cb(cmedia, pkt)
		})
	}
}

// OnPacketRTP sets the callback that is called when a RTP packet is read.
func (ss *ServerSession) OnPacketRTP(medi *media.Media, trak format.Format, cb func(*rtp.Packet)) {
	sm := ss.setuppedMedias[medi]
	st := sm.formats[trak.PayloadType()]
	st.onPacketRTP = cb
}

// OnPacketRTCP sets the callback that is called when a RTCP packet is read.
func (ss *ServerSession) OnPacketRTCP(medi *media.Media, cb func(rtcp.Packet)) {
	sm := ss.setuppedMedias[medi]
	sm.onPacketRTCP = cb
}

func (ss *ServerSession) writePacketRTP(medi *media.Media, byts []byte) {
	sm := ss.setuppedMedias[medi]
	sm.writePacketRTP(byts)
}

// WritePacketRTP writes a RTP packet to the session.
func (ss *ServerSession) WritePacketRTP(medi *media.Media, pkt *rtp.Packet) {
	byts, err := pkt.Marshal()
	if err != nil {
		return
	}

	ss.writePacketRTP(medi, byts)
}

func (ss *ServerSession) writePacketRTCP(medi *media.Media, byts []byte) {
	sm := ss.setuppedMedias[medi]
	sm.writePacketRTCP(byts)
}

// WritePacketRTCP writes a RTCP packet to the session.
func (ss *ServerSession) WritePacketRTCP(medi *media.Media, pkt rtcp.Packet) {
	byts, err := pkt.Marshal()
	if err != nil {
		return
	}

	ss.writePacketRTCP(medi, byts)
}
