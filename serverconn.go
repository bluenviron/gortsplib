package gortsplib

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/pion/rtcp"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/liberrors"
	"github.com/aler9/gortsplib/pkg/multibuffer"
)

func getSessionID(header base.Header) string {
	if h, ok := header["Session"]; ok && len(h) == 1 {
		return h[0]
	}
	return ""
}

type readReq struct {
	req *base.Request
	res chan error
}

// ServerConn is a server-side RTSP connection.
type ServerConn struct {
	s    *Server
	conn net.Conn

	ctx        context.Context
	ctxCancel  func()
	remoteAddr *net.TCPAddr
	br         *bufio.Reader
	session    *ServerSession
	readFunc   func(readRequest chan readReq) error

	// in
	sessionRemove chan *ServerSession

	// out
	done chan struct{}
}

func newServerConn(
	s *Server,
	nconn net.Conn,
) *ServerConn {
	ctx, ctxCancel := context.WithCancel(s.ctx)

	conn := func() net.Conn {
		if s.TLSConfig != nil {
			return tls.Server(nconn, s.TLSConfig)
		}
		return nconn
	}()

	sc := &ServerConn{
		s:             s,
		conn:          conn,
		ctx:           ctx,
		ctxCancel:     ctxCancel,
		remoteAddr:    conn.RemoteAddr().(*net.TCPAddr),
		sessionRemove: make(chan *ServerSession),
		done:          make(chan struct{}),
	}

	sc.readFunc = sc.readFuncStandard

	s.wg.Add(1)
	go sc.run()

	return sc
}

// Close closes the ServerConn.
func (sc *ServerConn) Close() error {
	sc.ctxCancel()
	return nil
}

// NetConn returns the underlying net.Conn.
func (sc *ServerConn) NetConn() net.Conn {
	return sc.conn
}

func (sc *ServerConn) ip() net.IP {
	return sc.remoteAddr.IP
}

func (sc *ServerConn) zone() string {
	return sc.remoteAddr.Zone
}

func (sc *ServerConn) run() {
	defer sc.s.wg.Done()
	defer close(sc.done)

	if h, ok := sc.s.Handler.(ServerHandlerOnConnOpen); ok {
		h.OnConnOpen(&ServerHandlerOnConnOpenCtx{
			Conn: sc,
		})
	}

	sc.br = bufio.NewReaderSize(sc.conn, serverReadBufferSize)

	readRequest := make(chan readReq)
	readErr := make(chan error)
	readDone := make(chan struct{})
	go sc.runReader(readRequest, readErr, readDone)

	err := sc.runInner(readRequest, readErr)

	sc.ctxCancel()

	sc.conn.Close()
	<-readDone

	if sc.session != nil {
		select {
		case sc.session.connRemove <- sc:
		case <-sc.session.ctx.Done():
		}
	}

	select {
	case sc.s.connClose <- sc:
	case <-sc.s.ctx.Done():
	}

	if h, ok := sc.s.Handler.(ServerHandlerOnConnClose); ok {
		h.OnConnClose(&ServerHandlerOnConnCloseCtx{
			Conn:  sc,
			Error: err,
		})
	}
}

func (sc *ServerConn) runInner(readRequest chan readReq, readErr chan error) error {
	for {
		select {
		case req := <-readRequest:
			req.res <- sc.handleRequestOuter(req.req)

		case err := <-readErr:
			return err

		case ss := <-sc.sessionRemove:
			if sc.session == ss {
				sc.session = nil
			}

		case <-sc.ctx.Done():
			return liberrors.ErrServerTerminated{}
		}
	}
}

var errSwitchReadFunc = errors.New("switch read function")

func (sc *ServerConn) runReader(readRequest chan readReq, readErr chan error, readDone chan struct{}) {
	defer close(readDone)

	for {
		err := sc.readFunc(readRequest)

		if err == errSwitchReadFunc {
			continue
		}

		select {
		case readErr <- err:
		case <-sc.ctx.Done():
		}
		break
	}
}

func (sc *ServerConn) readFuncStandard(readRequest chan readReq) error {
	// reset deadline
	sc.conn.SetReadDeadline(time.Time{})

	var req base.Request

	for {
		err := req.Read(sc.br)
		if err != nil {
			return err
		}

		cres := make(chan error)
		select {
		case readRequest <- readReq{req: &req, res: cres}:
			err = <-cres
			if err != nil {
				return err
			}

		case <-sc.ctx.Done():
			return liberrors.ErrServerTerminated{}
		}
	}
}

func (sc *ServerConn) readFuncTCP(readRequest chan readReq) error {
	// reset deadline
	sc.conn.SetReadDeadline(time.Time{})

	select {
	case sc.session.startWriter <- struct{}{}:
	case <-sc.session.ctx.Done():
	}

	var tcpReadBuffer *multibuffer.MultiBuffer
	var processFunc func(int, bool, []byte)

	if sc.session.state == ServerSessionStatePlay {
		// when playing, tcpReadBuffer is only used to receive RTCP receiver reports,
		// that are much smaller than RTP packets and are sent at a fixed interval.
		// decrease RAM consumption by allocating less buffers.
		tcpReadBuffer = multibuffer.New(8, uint64(sc.s.ReadBufferSize))

		processFunc = func(trackID int, isRTP bool, payload []byte) {
			if !isRTP {
				packets, err := rtcp.Unmarshal(payload)
				if err != nil {
					return
				}

				if h, ok := sc.s.Handler.(ServerHandlerOnPacketRTCP); ok {
					for _, pkt := range packets {
						h.OnPacketRTCP(&ServerHandlerOnPacketRTCPCtx{
							Session: sc.session,
							TrackID: trackID,
							Packet:  pkt,
						})
					}
				}
			}
		}
	} else {
		tcpReadBuffer = multibuffer.New(uint64(sc.s.ReadBufferCount), uint64(sc.s.ReadBufferSize))
		tcpRTPPacketBuffer := newRTPPacketMultiBuffer(uint64(sc.s.ReadBufferCount))

		processFunc = func(trackID int, isRTP bool, payload []byte) {
			if isRTP {
				pkt := tcpRTPPacketBuffer.next()
				err := pkt.Unmarshal(payload)
				if err != nil {
					return
				}

				sc.session.onPacketRTP(time.Time{}, trackID, pkt)
			} else {
				packets, err := rtcp.Unmarshal(payload)
				if err != nil {
					return
				}

				for _, pkt := range packets {
					sc.session.onPacketRTCP(trackID, pkt)
				}
			}
		}
	}

	var req base.Request
	var frame base.InterleavedFrame

	for {
		if sc.session.state == ServerSessionStateRecord {
			sc.conn.SetReadDeadline(time.Now().Add(sc.s.ReadTimeout))
		}

		frame.Payload = tcpReadBuffer.Next()
		what, err := base.ReadInterleavedFrameOrRequest(&frame, &req, sc.br)
		if err != nil {
			return err
		}

		switch what.(type) {
		case *base.InterleavedFrame:
			channel := frame.Channel
			isRTP := true
			if (channel % 2) != 0 {
				channel--
				isRTP = false
			}

			// forward frame only if it has been set up
			if trackID, ok := sc.session.tcpTracksByChannel[channel]; ok {
				processFunc(trackID, isRTP, frame.Payload)
			}

		case *base.Request:
			cres := make(chan error)
			select {
			case readRequest <- readReq{req: &req, res: cres}:
				err := <-cres
				if err != nil {
					return err
				}

			case <-sc.ctx.Done():
				return liberrors.ErrServerTerminated{}
			}
		}
	}
}

func (sc *ServerConn) handleRequest(req *base.Request) (*base.Response, error) {
	if cseq, ok := req.Header["CSeq"]; !ok || len(cseq) != 1 {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
			Header:     base.Header{},
		}, liberrors.ErrServerCSeqMissing{}
	}

	sxID := getSessionID(req.Header)

	switch req.Method {
	case base.Options:
		if sxID != "" {
			return sc.handleRequestInSession(sxID, req, false)
		}

		// handle request here
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

	case base.Describe:
		if h, ok := sc.s.Handler.(ServerHandlerOnDescribe); ok {
			pathAndQuery, ok := req.URL.RTSPPathAndQuery()
			if !ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerInvalidPath{}
			}

			path, query := base.PathSplitQuery(pathAndQuery)

			res, stream, err := h.OnDescribe(&ServerHandlerOnDescribeCtx{
				Conn:    sc,
				Request: req,
				Path:    path,
				Query:   query,
			})

			if res.StatusCode == base.StatusOK {
				if res.Header == nil {
					res.Header = make(base.Header)
				}

				res.Header["Content-Base"] = base.HeaderValue{req.URL.String() + "/"}
				res.Header["Content-Type"] = base.HeaderValue{"application/sdp"}

				// VLC uses multicast if the SDP contains a multicast address.
				// therefore, we introduce a special query (vlcmulticast) that allows
				// to return a SDP that contains a multicast address.
				multicast := false
				if sc.s.MulticastIPRange != "" {
					if q, err := url.ParseQuery(query); err == nil {
						if _, ok := q["vlcmulticast"]; ok {
							multicast = true
						}
					}
				}

				if stream != nil {
					res.Body = stream.Tracks().Write(multicast)
				}
			}

			return res, err
		}

	case base.Announce:
		if _, ok := sc.s.Handler.(ServerHandlerOnAnnounce); ok {
			return sc.handleRequestInSession(sxID, req, true)
		}

	case base.Setup:
		if _, ok := sc.s.Handler.(ServerHandlerOnSetup); ok {
			return sc.handleRequestInSession(sxID, req, true)
		}

	case base.Play:
		if sxID != "" {
			if _, ok := sc.s.Handler.(ServerHandlerOnPlay); ok {
				return sc.handleRequestInSession(sxID, req, false)
			}
		}

	case base.Record:
		if sxID != "" {
			if _, ok := sc.s.Handler.(ServerHandlerOnRecord); ok {
				return sc.handleRequestInSession(sxID, req, false)
			}
		}

	case base.Pause:
		if sxID != "" {
			if _, ok := sc.s.Handler.(ServerHandlerOnPause); ok {
				return sc.handleRequestInSession(sxID, req, false)
			}
		}

	case base.Teardown:
		if sxID != "" {
			return sc.handleRequestInSession(sxID, req, false)
		}

	case base.GetParameter:
		if sxID != "" {
			return sc.handleRequestInSession(sxID, req, false)
		}

		// handle request here
		if h, ok := sc.s.Handler.(ServerHandlerOnGetParameter); ok {
			pathAndQuery, ok := req.URL.RTSPPathAndQuery()
			if !ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerInvalidPath{}
			}

			path, query := base.PathSplitQuery(pathAndQuery)

			return h.OnGetParameter(&ServerHandlerOnGetParameterCtx{
				Conn:    sc,
				Request: req,
				Path:    path,
				Query:   query,
			})
		}

	case base.SetParameter:
		if h, ok := sc.s.Handler.(ServerHandlerOnSetParameter); ok {
			pathAndQuery, ok := req.URL.RTSPPathAndQuery()
			if !ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerInvalidPath{}
			}

			path, query := base.PathSplitQuery(pathAndQuery)

			return h.OnSetParameter(&ServerHandlerOnSetParameterCtx{
				Conn:    sc,
				Request: req,
				Path:    path,
				Query:   query,
			})
		}
	}

	return &base.Response{
		StatusCode: base.StatusBadRequest,
	}, liberrors.ErrServerUnhandledRequest{Request: req}
}

func (sc *ServerConn) handleRequestOuter(req *base.Request) error {
	if h, ok := sc.s.Handler.(ServerHandlerOnRequest); ok {
		h.OnRequest(sc, req)
	}

	res, err := sc.handleRequest(req)

	if res.Header == nil {
		res.Header = make(base.Header)
	}

	// add cseq
	if _, ok := err.(liberrors.ErrServerCSeqMissing); !ok {
		res.Header["CSeq"] = req.Header["CSeq"]
	}

	// add server
	res.Header["Server"] = base.HeaderValue{"gortsplib"}

	if h, ok := sc.s.Handler.(ServerHandlerOnResponse); ok {
		h.OnResponse(sc, res)
	}

	var buf bytes.Buffer
	res.Write(&buf)

	sc.conn.SetWriteDeadline(time.Now().Add(sc.s.WriteTimeout))
	sc.conn.Write(buf.Bytes())

	return err
}

func (sc *ServerConn) handleRequestInSession(
	sxID string,
	req *base.Request,
	create bool,
) (*base.Response, error) {
	// handle directly in Session
	if sc.session != nil {
		// session ID is optional in SETUP and ANNOUNCE requests, since
		// client may not have received the session ID yet due to multiple reasons:
		// * requests can be retries after code 301
		// * SETUP requests comes after ANNOUNCE response, that don't contain the session ID
		if sxID != "" {
			// the connection can't communicate with two sessions at once.
			if sxID != sc.session.secretID {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerLinkedToOtherSession{}
			}
		}

		cres := make(chan sessionRequestRes)
		sreq := sessionRequestReq{
			sc:     sc,
			req:    req,
			id:     sxID,
			create: create,
			res:    cres,
		}

		select {
		case sc.session.request <- sreq:
			res := <-cres
			sc.session = res.ss
			return res.res, res.err

		case <-sc.session.ctx.Done():
			return &base.Response{
				StatusCode: base.StatusBadRequest,
			}, liberrors.ErrServerTerminated{}
		}
	}

	// otherwise, pass through Server
	cres := make(chan sessionRequestRes)
	sreq := sessionRequestReq{
		sc:     sc,
		req:    req,
		id:     sxID,
		create: create,
		res:    cres,
	}

	select {
	case sc.s.sessionRequest <- sreq:
		res := <-cres
		sc.session = res.ss
		return res.res, res.err

	case <-sc.s.ctx.Done():
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, liberrors.ErrServerTerminated{}
	}
}
