package gortsplib

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
	"github.com/bluenviron/gortsplib/v4/pkg/rtptime"
	"github.com/bluenviron/gortsplib/v4/pkg/sdp"
)

type readFunc func([]byte)

func stringsReverseIndex(s, substr string) int {
	for i := len(s) - 1 - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func serverParseURLForPlay(u *base.URL) (string, string, string, error) {
	pathAndQuery, ok := u.RTSPPathAndQuery()
	if !ok {
		return "", "", "", liberrors.ErrServerInvalidPath{}
	}

	i := stringsReverseIndex(pathAndQuery, "/trackID=")
	if i < 0 {
		if !strings.HasSuffix(pathAndQuery, "/") {
			return "", "", "", liberrors.ErrServerPathNoSlash{}
		}

		path, query := base.PathSplitQuery(pathAndQuery[:len(pathAndQuery)-1])
		return path, query, "", nil
	}

	var trackID string
	pathAndQuery, trackID = pathAndQuery[:i], pathAndQuery[i+len("/trackID="):]
	path, query := base.PathSplitQuery(pathAndQuery)
	return path, query, trackID, nil
}

func recordBaseURL(u *base.URL, path string, query string) *base.URL {
	baseURL := &base.URL{
		Scheme:   u.Scheme,
		Host:     u.Host,
		Path:     path,
		RawQuery: query,
	}

	if baseURL.RawQuery != "" {
		baseURL.RawQuery += "/"
	} else {
		baseURL.Path += "/"
	}

	return baseURL
}

func findMediaByURL(medias []*description.Media, baseURL *base.URL, u *base.URL) *description.Media {
	for _, media := range medias {
		u1, err := media.URL(baseURL)
		if err == nil && u1.String() == u.String() {
			return media
		}
	}

	return nil
}

func findMediaByTrackID(medias []*description.Media, trackID string) *description.Media {
	if trackID == "" {
		return medias[0]
	}

	tmp, err := strconv.ParseUint(trackID, 10, 31)
	if err != nil {
		return nil
	}
	id := int(tmp)

	if len(medias) <= id {
		return nil
	}

	return medias[id]
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

func generateRTPInfo(
	now time.Time,
	setuppedMediasOrdered []*serverSessionMedia,
	setuppedStream *ServerStream,
	setuppedPath string,
	u *base.URL,
) (headers.RTPInfo, bool) {
	var ri headers.RTPInfo

	for _, sm := range setuppedMediasOrdered {
		entry := setuppedStream.rtpInfoEntry(sm.media, now)
		if entry != nil {
			entry.URL = (&base.URL{
				Scheme: u.Scheme,
				Host:   u.Host,
				Path: setuppedPath + "/trackID=" +
					strconv.FormatInt(int64(setuppedStream.streamMedias[sm.media].trackID), 10),
			}).String()
			ri = append(ri, entry)
		}
	}

	if len(ri) == 0 {
		return nil, false
	}

	return ri, true
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
	setuppedMedias        map[*description.Media]*serverSessionMedia
	setuppedMediasOrdered []*serverSessionMedia
	tcpCallbackByChannel  map[int]readFunc
	setuppedTransport     *Transport
	setuppedStream        *ServerStream // read
	setuppedPath          string
	setuppedQuery         string
	lastRequestTime       time.Time
	tcpConn               *ServerConn
	announcedDesc         *description.Session // publish
	udpLastPacketTime     *int64               // publish
	udpCheckStreamTimer   *time.Timer
	writer                asyncProcessor
	timeDecoder           *rtptime.GlobalDecoder

	// in
	chHandleRequest chan sessionRequestReq
	chRemoveConn    chan *ServerConn
	chStartWriter   chan struct{}
}

func newServerSession(
	s *Server,
	author *ServerConn,
) *ServerSession {
	ctx, ctxCancel := context.WithCancel(s.ctx)

	// use an UUID without dashes, since dashes confuse some clients.
	secretID := strings.ReplaceAll(uuid.New().String(), "-", "")

	ss := &ServerSession{
		s:                   s,
		secretID:            secretID,
		author:              author,
		ctx:                 ctx,
		ctxCancel:           ctxCancel,
		bytesReceived:       new(uint64),
		bytesSent:           new(uint64),
		conns:               make(map[*ServerConn]struct{}),
		lastRequestTime:     s.timeNow(),
		udpCheckStreamTimer: emptyTimer(),
		chHandleRequest:     make(chan sessionRequestReq),
		chRemoveConn:        make(chan *ServerConn),
		chStartWriter:       make(chan struct{}),
	}

	s.wg.Add(1)
	go ss.run()

	return ss
}

// Close closes the ServerSession.
func (ss *ServerSession) Close() {
	ss.ctxCancel()
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

// SetuppedStream returns the stream associated with the session.
func (ss *ServerSession) SetuppedStream() *ServerStream {
	return ss.setuppedStream
}

// SetuppedPath returns the path sent during SETUP or ANNOUNCE.
func (ss *ServerSession) SetuppedPath() string {
	return ss.setuppedPath
}

// SetuppedQuery returns the query sent during SETUP or ANNOUNCE.
func (ss *ServerSession) SetuppedQuery() string {
	return ss.setuppedQuery
}

// AnnouncedDescription returns the announced stream description.
func (ss *ServerSession) AnnouncedDescription() *description.Session {
	return ss.announcedDesc
}

// SetuppedMedias returns the setupped medias.
func (ss *ServerSession) SetuppedMedias() []*description.Media {
	ret := make([]*description.Media, len(ss.setuppedMedias))
	for i, sm := range ss.setuppedMediasOrdered {
		ret[i] = sm.media
	}
	return ret
}

// SetUserData sets some user data associated to the session.
func (ss *ServerSession) SetUserData(v interface{}) {
	ss.userData = v
}

// UserData returns some user data associated to the session.
func (ss *ServerSession) UserData() interface{} {
	return ss.userData
}

func (ss *ServerSession) onPacketLost(err error) {
	if h, ok := ss.s.Handler.(ServerHandlerOnPacketLost); ok {
		h.OnPacketLost(&ServerHandlerOnPacketLostCtx{
			Session: ss,
			Error:   err,
		})
	} else {
		log.Println(err.Error())
	}
}

func (ss *ServerSession) onDecodeError(err error) {
	if h, ok := ss.s.Handler.(ServerHandlerOnDecodeError); ok {
		h.OnDecodeError(&ServerHandlerOnDecodeErrorCtx{
			Session: ss,
			Error:   err,
		})
	} else {
		log.Println(err.Error())
	}
}

func (ss *ServerSession) onStreamWriteError(err error) {
	if h, ok := ss.s.Handler.(ServerHandlerOnStreamWriteError); ok {
		h.OnStreamWriteError(&ServerHandlerOnStreamWriteErrorCtx{
			Session: ss,
			Error:   err,
		})
	} else {
		log.Println(err.Error())
	}
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

	// close all associated connections, both UDP and TCP
	// except for the ones that called TEARDOWN
	// (that are detached from the session just after the request)
	for sc := range ss.conns {
		sc.Close()

		// make sure that OnFrame() is never called after OnSessionClose()
		<-sc.done

		sc.removeSession(ss)
	}

	if ss.setuppedStream != nil {
		ss.setuppedStream.readerSetInactive(ss)
		ss.setuppedStream.readerRemove(ss)
	}

	ss.writer.stop()

	for _, sm := range ss.setuppedMedias {
		sm.stop()
	}

	ss.s.closeSession(ss)

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
		case req := <-ss.chHandleRequest:
			ss.lastRequestTime = ss.s.timeNow()

			if _, ok := ss.conns[req.sc]; !ok {
				ss.conns[req.sc] = struct{}{}
			}

			res, err := ss.handleRequestInner(req.sc, req.req)

			returnedSession := ss

			if err == nil || isErrSwitchReadFunc(err) {
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

			if (err == nil || isErrSwitchReadFunc(err)) && savedMethod == base.Teardown {
				return liberrors.ErrServerSessionTornDown{Author: req.sc.NetConn().RemoteAddr()}
			}

		case sc := <-ss.chRemoveConn:
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

		case <-ss.chStartWriter:
			if (ss.state == ServerSessionStateRecord ||
				ss.state == ServerSessionStatePlay) &&
				*ss.setuppedTransport == TransportTCP {
				ss.writer.start()
			}

		case <-ss.udpCheckStreamTimer.C:
			now := ss.s.timeNow()

			lft := atomic.LoadInt64(ss.udpLastPacketTime)

			// in case of RECORD, timeout happens when no RTP or RTCP packets are being received
			if ss.state == ServerSessionStateRecord {
				if now.Sub(time.Unix(lft, 0)) >= ss.s.ReadTimeout {
					return liberrors.ErrServerSessionTimedOut{}
				}

				// in case of PLAY, timeout happens when no RTSP keepalives and no RTCP packets are being received
			} else if now.Sub(ss.lastRequestTime) >= ss.s.sessionTimeout &&
				now.Sub(time.Unix(lft, 0)) >= ss.s.sessionTimeout {
				return liberrors.ErrServerSessionTimedOut{}
			}

			ss.udpCheckStreamTimer = time.NewTimer(ss.s.checkStreamPeriod)

		case <-ss.ctx.Done():
			return liberrors.ErrServerTerminated{}
		}
	}
}

func (ss *ServerSession) handleRequestInner(sc *ServerConn, req *base.Request) (*base.Response, error) {
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

		// pathAndQuery can end with a slash due to Content-Base, remove it
		if ss.state == ServerSessionStatePrePlay || ss.state == ServerSessionStatePlay {
			pathAndQuery = strings.TrimSuffix(pathAndQuery, "/")
		}

		path, query = base.PathSplitQuery(pathAndQuery)
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

		var ssd sdp.SessionDescription
		err = ssd.Unmarshal(req.Body)
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerSDPInvalid{Err: err}
		}

		var desc description.Session
		err = desc.Unmarshal(&ssd)
		if err != nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerSDPInvalid{Err: err}
		}

		for _, medi := range desc.Medias {
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
			Session:     ss,
			Conn:        sc,
			Request:     req,
			Path:        path,
			Query:       query,
			Description: &desc,
		})

		if res.StatusCode != base.StatusOK {
			return res, err
		}

		ss.state = ServerSessionStatePreRecord
		ss.setuppedPath = path
		ss.setuppedQuery = query
		ss.announcedDesc = &desc

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
		var trackID string
		switch ss.state {
		case ServerSessionStateInitial, ServerSessionStatePrePlay: // play
			var err error
			path, query, trackID, err = serverParseURLForPlay(req.URL)
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, err
			}

			if ss.state == ServerSessionStatePrePlay && path != ss.setuppedPath {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerMediasDifferentPaths{}
			}

		default: // record
			path = ss.setuppedPath
			query = ss.setuppedQuery
		}

		var transport Transport

		if inTH.Protocol == headers.TransportProtocolUDP {
			if inTH.Delivery != nil && *inTH.Delivery == headers.TransportDeliveryMulticast {
				transport = TransportUDPMulticast
			} else {
				transport = TransportUDP

				if inTH.ClientPorts == nil {
					return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, liberrors.ErrServerTransportHeaderNoClientPorts{}
				}
			}
		} else {
			transport = TransportTCP

			if inTH.InterleavedIDs != nil {
				if (inTH.InterleavedIDs[0] + 1) != inTH.InterleavedIDs[1] {
					return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, liberrors.ErrServerTransportHeaderInvalidInterleavedIDs{}
				}

				if ss.isChannelPairInUse(inTH.InterleavedIDs[0]) {
					return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, liberrors.ErrServerTransportHeaderInterleavedIDsInUse{}
				}
			}
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

		var medi *description.Media
		switch ss.state {
		case ServerSessionStateInitial, ServerSessionStatePrePlay: // play
			medi = findMediaByTrackID(stream.desc.Medias, trackID)
		default: // record
			medi = findMediaByURL(ss.announcedDesc.Medias, recordBaseURL(req.URL, path, query), req.URL)
		}

		if medi == nil {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerMediaNotFound{}
		}

		if _, ok := ss.setuppedMedias[medi]; ok {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerMediaAlreadySetup{}
		}

		ss.setuppedTransport = &transport

		if ss.state == ServerSessionStateInitial {
			err := stream.readerAdd(ss,
				inTH.ClientPorts,
			)
			if err != nil {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, err
			}

			ss.state = ServerSessionStatePrePlay
			ss.setuppedPath = path
			ss.setuppedQuery = query
			ss.setuppedStream = stream
		}

		th := headers.Transport{}

		if ss.state == ServerSessionStatePrePlay {
			ssrc, ok := stream.senderSSRC(medi)
			if ok {
				th.SSRC = &ssrc
			}
		}

		if res.Header == nil {
			res.Header = make(base.Header)
		}

		sm := newServerSessionMedia(ss, medi)

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
			d := stream.streamMedias[medi].multicastWriter.ip()
			th.Destination = &d
			th.Ports = &[2]int{ss.s.MulticastRTPPort, ss.s.MulticastRTCPPort}

		default: // TCP
			if inTH.InterleavedIDs != nil {
				sm.tcpChannel = inTH.InterleavedIDs[0]
			} else {
				sm.tcpChannel = ss.findFreeChannelPair()
			}

			th.Protocol = headers.TransportProtocolTCP
			de := headers.TransportDeliveryUnicast
			th.Delivery = &de
			th.InterleavedIDs = &[2]int{sm.tcpChannel, sm.tcpChannel + 1}
		}

		if ss.setuppedMedias == nil {
			ss.setuppedMedias = make(map[*description.Media]*serverSessionMedia)
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

		if ss.State() == ServerSessionStatePrePlay && path != ss.setuppedPath {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerPathHasChanged{Prev: ss.setuppedPath, Cur: path}
		}

		// allocate writeBuffer before calling OnPlay().
		// in this way it's possible to call ServerSession.WritePacket*()
		// inside the callback.
		if ss.state != ServerSessionStatePlay &&
			*ss.setuppedTransport != TransportUDPMulticast {
			ss.writer.allocateBuffer(ss.s.WriteQueueSize)
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

		v := ss.s.timeNow().Unix()
		ss.udpLastPacketTime = &v

		ss.timeDecoder = rtptime.NewGlobalDecoder()

		for _, sm := range ss.setuppedMedias {
			sm.start()
		}

		switch *ss.setuppedTransport {
		case TransportUDP:
			ss.udpCheckStreamTimer = time.NewTimer(ss.s.checkStreamPeriod)
			ss.writer.start()

		case TransportUDPMulticast:
			ss.udpCheckStreamTimer = time.NewTimer(ss.s.checkStreamPeriod)

		default: // TCP
			ss.tcpConn = sc
			err = errSwitchReadFunc{true}
			// writer.start() is called by ServerConn after the response has been sent
		}

		ss.setuppedStream.readerSetActive(ss)

		rtpInfo, ok := generateRTPInfo(
			ss.s.timeNow(),
			ss.setuppedMediasOrdered,
			ss.setuppedStream,
			ss.setuppedPath,
			req.URL)

		if ok {
			if res.Header == nil {
				res.Header = make(base.Header)
			}
			res.Header["RTP-Info"] = rtpInfo.Marshal()
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

		if len(ss.setuppedMedias) != len(ss.announcedDesc.Medias) {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerNotAllAnnouncedMediasSetup{}
		}

		if path != ss.setuppedPath {
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerPathHasChanged{Prev: ss.setuppedPath, Cur: path}
		}

		// allocate writeBuffer before calling OnRecord().
		// in this way it's possible to call ServerSession.WritePacket*()
		// inside the callback.
		// when recording, writeBuffer is only used to send RTCP receiver reports,
		// that are much smaller than RTP packets and are sent at a fixed interval.
		// decrease RAM consumption by allocating less buffers.
		ss.writer.allocateBuffer(8)

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

		v := ss.s.timeNow().Unix()
		ss.udpLastPacketTime = &v

		ss.timeDecoder = rtptime.NewGlobalDecoder()

		for _, sm := range ss.setuppedMedias {
			sm.start()
		}

		switch *ss.setuppedTransport {
		case TransportUDP:
			ss.udpCheckStreamTimer = time.NewTimer(ss.s.checkStreamPeriod)
			ss.writer.start()

		default: // TCP
			ss.tcpConn = sc
			err = errSwitchReadFunc{true}
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

		if ss.setuppedStream != nil {
			ss.setuppedStream.readerSetInactive(ss)
		}

		ss.writer.stop()

		for _, sm := range ss.setuppedMedias {
			sm.stop()
		}

		ss.timeDecoder = nil

		switch ss.state {
		case ServerSessionStatePlay:
			ss.state = ServerSessionStatePrePlay

			switch *ss.setuppedTransport {
			case TransportUDP:
				ss.udpCheckStreamTimer = emptyTimer()

			case TransportUDPMulticast:
				ss.udpCheckStreamTimer = emptyTimer()

			default: // TCP
				err = errSwitchReadFunc{false}
				ss.tcpConn = nil
			}

		case ServerSessionStateRecord:
			switch *ss.setuppedTransport {
			case TransportUDP:
				ss.udpCheckStreamTimer = emptyTimer()

			default: // TCP
				err = errSwitchReadFunc{false}
				ss.tcpConn = nil
			}

			ss.state = ServerSessionStatePreRecord
		}

		return res, err

	case base.Teardown:
		var err error
		if (ss.state == ServerSessionStatePlay || ss.state == ServerSessionStateRecord) &&
			*ss.setuppedTransport == TransportTCP {
			err = errSwitchReadFunc{false}
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

func (ss *ServerSession) isChannelPairInUse(channel int) bool {
	for _, sm := range ss.setuppedMedias {
		if (sm.tcpChannel+1) == channel || sm.tcpChannel == channel || sm.tcpChannel == (channel+1) {
			return true
		}
	}
	return false
}

func (ss *ServerSession) findFreeChannelPair() int {
	for i := 0; ; i += 2 { // prefer even channels
		if !ss.isChannelPairInUse(i) {
			return i
		}
	}
}

// OnPacketRTPAny sets the callback that is called when a RTP packet is read from any setupped media.
func (ss *ServerSession) OnPacketRTPAny(cb OnPacketRTPAnyFunc) {
	for _, sm := range ss.setuppedMedias {
		cmedia := sm.media
		for _, forma := range sm.media.Formats {
			ss.OnPacketRTP(sm.media, forma, func(pkt *rtp.Packet) {
				cb(cmedia, forma, pkt)
			})
		}
	}
}

// OnPacketRTCPAny sets the callback that is called when a RTCP packet is read from any setupped media.
func (ss *ServerSession) OnPacketRTCPAny(cb OnPacketRTCPAnyFunc) {
	for _, sm := range ss.setuppedMedias {
		cmedia := sm.media
		ss.OnPacketRTCP(sm.media, func(pkt rtcp.Packet) {
			cb(cmedia, pkt)
		})
	}
}

// OnPacketRTP sets the callback that is called when a RTP packet is read.
func (ss *ServerSession) OnPacketRTP(medi *description.Media, forma format.Format, cb OnPacketRTPFunc) {
	sm := ss.setuppedMedias[medi]
	st := sm.formats[forma.PayloadType()]
	st.onPacketRTP = cb
}

// OnPacketRTCP sets the callback that is called when a RTCP packet is read.
func (ss *ServerSession) OnPacketRTCP(medi *description.Media, cb OnPacketRTCPFunc) {
	sm := ss.setuppedMedias[medi]
	sm.onPacketRTCP = cb
}

func (ss *ServerSession) writePacketRTP(medi *description.Media, byts []byte) error {
	sm := ss.setuppedMedias[medi]
	return sm.writePacketRTP(byts)
}

// WritePacketRTP writes a RTP packet to the session.
func (ss *ServerSession) WritePacketRTP(medi *description.Media, pkt *rtp.Packet) error {
	byts := make([]byte, ss.s.MaxPacketSize)
	n, err := pkt.MarshalTo(byts)
	if err != nil {
		return err
	}
	byts = byts[:n]

	return ss.writePacketRTP(medi, byts)
}

func (ss *ServerSession) writePacketRTCP(medi *description.Media, byts []byte) error {
	sm := ss.setuppedMedias[medi]
	return sm.writePacketRTCP(byts)
}

// WritePacketRTCP writes a RTCP packet to the session.
func (ss *ServerSession) WritePacketRTCP(medi *description.Media, pkt rtcp.Packet) error {
	byts, err := pkt.Marshal()
	if err != nil {
		return err
	}

	return ss.writePacketRTCP(medi, byts)
}

// PacketPTS returns the PTS of an incoming RTP packet.
// It is computed by decoding the packet timestamp and sychronizing it with other tracks.
func (ss *ServerSession) PacketPTS(medi *description.Media, pkt *rtp.Packet) (time.Duration, bool) {
	sm := ss.setuppedMedias[medi]
	sf := sm.formats[pkt.PayloadType]
	return ss.timeDecoder.Decode(sf.format, pkt)
}

// PacketNTP returns the NTP timestamp of an incoming RTP packet.
// The NTP timestamp is computed from sender reports.
func (ss *ServerSession) PacketNTP(medi *description.Media, pkt *rtp.Packet) (time.Time, bool) {
	sm := ss.setuppedMedias[medi]
	sf := sm.formats[pkt.PayloadType]
	return sf.rtcpReceiver.PacketNTP(pkt.Timestamp)
}

func (ss *ServerSession) handleRequest(req sessionRequestReq) (*base.Response, *ServerSession, error) {
	select {
	case ss.chHandleRequest <- req:
		res := <-req.res
		return res.res, res.ss, res.err

	case <-ss.ctx.Done():
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, req.sc.session, liberrors.ErrServerTerminated{}
	}
}

func (ss *ServerSession) removeConn(sc *ServerConn) {
	select {
	case ss.chRemoveConn <- sc:
	case <-ss.ctx.Done():
	}
}

func (ss *ServerSession) startWriter() {
	select {
	case ss.chStartWriter <- struct{}{}:
	case <-ss.ctx.Done():
	}
}
