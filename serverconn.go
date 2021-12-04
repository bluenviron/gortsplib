package gortsplib

import (
	"bufio"
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/liberrors"
	"github.com/aler9/gortsplib/pkg/multibuffer"
	"github.com/aler9/gortsplib/pkg/ringbuffer"
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
	s     *Server
	nconn net.Conn

	ctx                         context.Context
	ctxCancel                   func()
	remoteAddr                  *net.TCPAddr // to improve speed
	br                          *bufio.Reader
	bw                          *bufio.Writer
	sessions                    map[string]*ServerSession
	tcpFrameSetEnabled          bool                     // tcp
	tcpFrameEnabled             bool                     // tcp
	tcpSession                  *ServerSession           // tcp
	tcpFrameIsRecording         bool                     // tcp
	tcpFrameTimeout             bool                     // tcp
	tcpReadBuffer               *multibuffer.MultiBuffer // tcp
	tcpFrameWriteBuffer         *ringbuffer.RingBuffer   // tcp
	tcpFrameBackgroundWriteDone chan struct{}            // tcp
	tcpProcessFunc              func(int, bool, []byte)

	// in
	sessionRemove chan *ServerSession

	// out
	done chan struct{}
}

func newServerConn(
	s *Server,
	nconn net.Conn) *ServerConn {
	ctx, ctxCancel := context.WithCancel(s.ctx)

	sc := &ServerConn{
		s:             s,
		nconn:         nconn,
		ctx:           ctx,
		ctxCancel:     ctxCancel,
		remoteAddr:    nconn.RemoteAddr().(*net.TCPAddr),
		sessionRemove: make(chan *ServerSession),
		done:          make(chan struct{}),
	}

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
	return sc.nconn
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

	conn := func() net.Conn {
		if sc.s.TLSConfig != nil {
			return tls.Server(sc.nconn, sc.s.TLSConfig)
		}
		return sc.nconn
	}()

	sc.br = bufio.NewReaderSize(conn, serverReadBufferSize)
	sc.bw = bufio.NewWriterSize(conn, serverWriteBufferSize)
	sc.sessions = make(map[string]*ServerSession)

	// instantiate always to allow writing to this conn before Play()
	sc.tcpFrameWriteBuffer = ringbuffer.New(uint64(sc.s.ReadBufferCount))

	readRequest := make(chan readReq)
	readErr := make(chan error)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		err := func() error {
			var req base.Request
			var frame base.InterleavedFrame

			for {
				if sc.tcpFrameEnabled {
					if sc.tcpFrameTimeout {
						sc.nconn.SetReadDeadline(time.Now().Add(sc.s.ReadTimeout))
					}

					frame.Payload = sc.tcpReadBuffer.Next()
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
						if trackID, ok := sc.tcpSession.setuppedTracksByChannel[channel]; ok {
							sc.tcpProcessFunc(trackID, isRTP, frame.Payload)
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
				} else {
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
		}()

		select {
		case readErr <- err:
		case <-sc.ctx.Done():
		}
	}()

	err := func() error {
		for {
			select {
			case req := <-readRequest:
				req.res <- sc.handleRequestOuter(req.req)

			case err := <-readErr:
				return err

			case ss := <-sc.sessionRemove:
				if _, ok := sc.sessions[ss.secretID]; ok {
					delete(sc.sessions, ss.secretID)

					select {
					case ss.connRemove <- sc:
					case <-ss.ctx.Done():
					}
				}

			case <-sc.ctx.Done():
				return liberrors.ErrServerTerminated{}
			}
		}
	}()

	sc.ctxCancel()

	if sc.tcpFrameEnabled {
		sc.tcpFrameWriteBuffer.Close()
		<-sc.tcpFrameBackgroundWriteDone
	}

	sc.nconn.Close()
	<-readDone

	for _, ss := range sc.sessions {
		select {
		case ss.connRemove <- sc:
		case <-ss.ctx.Done():
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

func (sc *ServerConn) tcpProcessPlay(trackID int, isRTP bool, payload []byte) {
	if !isRTP {
		if h, ok := sc.s.Handler.(ServerHandlerOnPacketRTCP); ok {
			h.OnPacketRTCP(&ServerHandlerOnPacketRTCPCtx{
				Session: sc.tcpSession,
				TrackID: trackID,
				Payload: payload,
			})
		}
	}
}

func (sc *ServerConn) tcpProcessRecord(trackID int, isRTP bool, payload []byte) {
	if isRTP {
		if h, ok := sc.s.Handler.(ServerHandlerOnPacketRTP); ok {
			h.OnPacketRTP(&ServerHandlerOnPacketRTPCtx{
				Session: sc.tcpSession,
				TrackID: trackID,
				Payload: payload,
			})
		}
	} else {
		if h, ok := sc.s.Handler.(ServerHandlerOnPacketRTCP); ok {
			h.OnPacketRTCP(&ServerHandlerOnPacketRTCPCtx{
				Session: sc.tcpSession,
				TrackID: trackID,
				Payload: payload,
			})
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

	// the connection can't communicate with another session
	// if it's receiving or sending TCP frames.
	if sc.tcpSession != nil &&
		sxID != sc.tcpSession.secretID {
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, liberrors.ErrServerLinkedToOtherSession{}
	}

	switch req.Method {
	case base.Options:
		// handle request in session
		if sxID != "" {
			_, res, err := sc.handleRequestInSession(sxID, req, false)
			return res, err
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
				Conn:  sc,
				Req:   req,
				Path:  path,
				Query: query,
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
			_, res, err := sc.handleRequestInSession(sxID, req, true)
			return res, err
		}

	case base.Setup:
		if _, ok := sc.s.Handler.(ServerHandlerOnSetup); ok {
			_, res, err := sc.handleRequestInSession(sxID, req, true)
			return res, err
		}

	case base.Play:
		if _, ok := sc.s.Handler.(ServerHandlerOnPlay); ok {
			ss, res, err := sc.handleRequestInSession(sxID, req, false)

			if _, ok := err.(liberrors.ErrServerTCPFramesEnable); ok {
				sc.tcpSession = ss
				sc.tcpFrameIsRecording = false
				sc.tcpFrameSetEnabled = true
				return res, nil
			}

			return res, err
		}

	case base.Record:
		if _, ok := sc.s.Handler.(ServerHandlerOnRecord); ok {
			ss, res, err := sc.handleRequestInSession(sxID, req, false)

			if _, ok := err.(liberrors.ErrServerTCPFramesEnable); ok {
				sc.tcpSession = ss
				sc.tcpFrameIsRecording = true
				sc.tcpFrameSetEnabled = true
				return res, nil
			}

			return res, err
		}

	case base.Pause:
		if _, ok := sc.s.Handler.(ServerHandlerOnPause); ok {
			_, res, err := sc.handleRequestInSession(sxID, req, false)

			if _, ok := err.(liberrors.ErrServerTCPFramesDisable); ok {
				sc.tcpSession = nil
				sc.tcpFrameSetEnabled = false
				return res, nil
			}

			return res, err
		}

	case base.Teardown:
		_, res, err := sc.handleRequestInSession(sxID, req, false)
		return res, err

	case base.GetParameter:
		// handle request in session
		if sxID != "" {
			_, res, err := sc.handleRequestInSession(sxID, req, false)
			return res, err
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
				Conn:  sc,
				Req:   req,
				Path:  path,
				Query: query,
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
				Conn:  sc,
				Req:   req,
				Path:  path,
				Query: query,
			})
		}
	}

	return &base.Response{
		StatusCode: base.StatusBadRequest,
	}, liberrors.ErrServerUnhandledRequest{Req: req}
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

	switch {
	case sc.tcpFrameSetEnabled != sc.tcpFrameEnabled:
		sc.tcpFrameEnabled = sc.tcpFrameSetEnabled

		// write response before frames
		sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.WriteTimeout))
		res.Write(sc.bw)

		if sc.tcpFrameEnabled {
			if sc.tcpFrameIsRecording {
				sc.tcpFrameTimeout = true
				sc.tcpReadBuffer = multibuffer.New(uint64(sc.s.ReadBufferCount), uint64(sc.s.ReadBufferSize))
				sc.tcpProcessFunc = sc.tcpProcessRecord
			} else {
				// when playing, tcpReadBuffer is only used to receive RTCP receiver reports,
				// that are much smaller than RTP packets and are sent at a fixed interval
				// (about 2 frames every 10 secs).
				// decrease RAM consumption by allocating less buffers.
				sc.tcpReadBuffer = multibuffer.New(8, uint64(sc.s.ReadBufferSize))
				sc.tcpProcessFunc = sc.tcpProcessPlay
			}

			// start background write
			sc.tcpFrameBackgroundWriteDone = make(chan struct{})
			go sc.tcpFrameBackgroundWrite()
		} else {
			if sc.tcpFrameIsRecording {
				sc.tcpFrameTimeout = false
				sc.nconn.SetReadDeadline(time.Time{})
			}

			sc.tcpFrameEnabled = false
			sc.tcpFrameWriteBuffer.Close()
			<-sc.tcpFrameBackgroundWriteDone
			sc.tcpFrameWriteBuffer.Reset()

			sc.tcpReadBuffer = nil
		}

	case sc.tcpFrameEnabled: // write to background write
		sc.tcpFrameWriteBuffer.Push(res)

	default: // write directly
		sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.WriteTimeout))
		res.Write(sc.bw)
	}

	return err
}

func (sc *ServerConn) handleRequestInSession(
	sxID string,
	req *base.Request,
	create bool,
) (*ServerSession, *base.Response, error) {
	// if the session is already linked to this conn, communicate directly with it
	if sxID != "" {
		if ss, ok := sc.sessions[sxID]; ok {
			cres := make(chan sessionRequestRes)
			sreq := sessionRequestReq{
				sc:     sc,
				req:    req,
				id:     sxID,
				create: create,
				res:    cres,
			}

			select {
			case ss.request <- sreq:
				res := <-cres
				return ss, res.res, res.err

			case <-ss.ctx.Done():
				return nil, &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerTerminated{}
			}
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
		if res.ss != nil {
			sc.sessions[res.ss.secretID] = res.ss
		}

		return res.ss, res.res, res.err

	case <-sc.s.ctx.Done():
		return nil, &base.Response{
			StatusCode: base.StatusBadRequest,
		}, liberrors.ErrServerTerminated{}
	}
}

func (sc *ServerConn) tcpFrameBackgroundWrite() {
	defer close(sc.tcpFrameBackgroundWriteDone)

	for {
		what, ok := sc.tcpFrameWriteBuffer.Pull()
		if !ok {
			return
		}

		switch w := what.(type) {
		case *base.InterleavedFrame:
			sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.WriteTimeout))
			w.Write(sc.bw)

		case *base.Response:
			sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.WriteTimeout))
			w.Write(sc.bw)
		}
	}
}
