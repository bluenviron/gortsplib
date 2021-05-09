package gortsplib

import (
	"bufio"
	"crypto/tls"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/liberrors"
	"github.com/aler9/gortsplib/pkg/multibuffer"
	"github.com/aler9/gortsplib/pkg/ringbuffer"
)

const (
	serverConnReadBufferSize  = 4096
	serverConnWriteBufferSize = 4096
)

func stringsReverseIndex(s, substr string) int {
	for i := len(s) - 1 - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

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
	wg    *sync.WaitGroup
	nconn net.Conn
	br    *bufio.Reader
	bw    *bufio.Writer

	sessions   map[string]*ServerSession
	sessionsWG sync.WaitGroup

	// TCP stream protocol
	tcpFrameSetEnabled          bool
	tcpFrameEnabled             bool
	tcpSession                  *ServerSession
	tcpFrameIsRecording         bool
	tcpFrameTimeout             bool
	tcpFrameBuffer              *multibuffer.MultiBuffer
	tcpFrameWriteBuffer         *ringbuffer.RingBuffer
	tcpFrameBackgroundWriteDone chan struct{}

	// in
	sessionRemove   chan *ServerSession
	terminate       chan struct{}
	parentTerminate chan struct{}
}

func newServerConn(
	s *Server,
	wg *sync.WaitGroup,
	nconn net.Conn) *ServerConn {

	sc := &ServerConn{
		s:               s,
		wg:              wg,
		nconn:           nconn,
		sessionRemove:   make(chan *ServerSession),
		terminate:       make(chan struct{}, 1),
		parentTerminate: make(chan struct{}),
	}

	wg.Add(1)
	go sc.run()

	return sc
}

// Close closes the ServerConn.
func (sc *ServerConn) Close() error {
	select {
	case sc.terminate <- struct{}{}:
	default:
	}
	return nil
}

// NetConn returns the underlying net.Conn.
func (sc *ServerConn) NetConn() net.Conn {
	return sc.nconn
}

func (sc *ServerConn) ip() net.IP {
	return sc.nconn.RemoteAddr().(*net.TCPAddr).IP
}

func (sc *ServerConn) zone() string {
	return sc.nconn.RemoteAddr().(*net.TCPAddr).Zone
}

func (sc *ServerConn) run() {
	defer sc.wg.Done()

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

	sc.br = bufio.NewReaderSize(conn, serverConnReadBufferSize)
	sc.bw = bufio.NewWriterSize(conn, serverConnWriteBufferSize)
	sc.sessions = make(map[string]*ServerSession)

	// instantiate always to allow writing to this conn before Play()
	sc.tcpFrameWriteBuffer = ringbuffer.New(uint64(sc.s.ReadBufferCount))

	readRequest := make(chan readReq)
	readErr := make(chan error)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		readErr <- func() error {
			var req base.Request
			var frame base.InterleavedFrame

			for {
				if sc.tcpFrameEnabled {
					if sc.tcpFrameTimeout {
						sc.nconn.SetReadDeadline(time.Now().Add(sc.s.ReadTimeout))
					}

					frame.Payload = sc.tcpFrameBuffer.Next()
					what, err := base.ReadInterleavedFrameOrRequest(&frame, &req, sc.br)
					if err != nil {
						return err
					}

					switch what.(type) {
					case *base.InterleavedFrame:
						// forward frame only if it has been set up
						if _, ok := sc.tcpSession.setuppedTracks[frame.TrackID]; ok {
							if sc.tcpFrameIsRecording {
								sc.tcpSession.announcedTracks[frame.TrackID].rtcpReceiver.ProcessFrame(time.Now(),
									frame.StreamType, frame.Payload)
							}

							if h, ok := sc.s.Handler.(ServerHandlerOnFrame); ok {
								h.OnFrame(&ServerHandlerOnFrameCtx{
									Session:    sc.tcpSession,
									TrackID:    frame.TrackID,
									StreamType: frame.StreamType,
									Payload:    frame.Payload,
								})
							}
						}

					case *base.Request:
						cres := make(chan error)
						readRequest <- readReq{req: &req, res: cres}
						err := <-cres
						if err != nil {
							return err
						}
					}

				} else {
					err := req.Read(sc.br)
					if err != nil {
						return err
					}

					cres := make(chan error)
					readRequest <- readReq{req: &req, res: cres}
					err = <-cres
					if err != nil {
						return err
					}
				}
			}
		}()
	}()

	err := func() error {
		for {
			select {
			case req := <-readRequest:
				req.res <- sc.handleRequestOuter(req.req)

			case err := <-readErr:
				return err

			case ss := <-sc.sessionRemove:
				if _, ok := sc.sessions[ss.ID()]; ok {
					delete(sc.sessions, ss.ID())
					ss.connRemove <- sc
					sc.sessionsWG.Done()
				}

			case <-sc.terminate:
				return liberrors.ErrServerTerminated{}
			}
		}
	}()

	go func() {
		for {
			select {
			case req, ok := <-readRequest:
				if !ok {
					return
				}

				req.res <- liberrors.ErrServerTerminated{}

			case _, ok := <-readErr:
				if !ok {
					return
				}

			case ss, ok := <-sc.sessionRemove:
				if !ok {
					return
				}

				if _, ok := sc.sessions[ss.ID()]; ok {
					sc.sessionsWG.Done()
				}
			}
		}
	}()

	if sc.tcpFrameEnabled {
		sc.tcpFrameWriteBuffer.Close()
		<-sc.tcpFrameBackgroundWriteDone
	}

	sc.nconn.Close()
	<-readDone

	for _, ss := range sc.sessions {
		ss.connRemove <- sc
	}
	sc.sessionsWG.Wait()

	sc.s.connClose <- sc
	<-sc.parentTerminate

	close(readRequest)
	close(readErr)
	close(sc.sessionRemove)

	if h, ok := sc.s.Handler.(ServerHandlerOnConnClose); ok {
		h.OnConnClose(&ServerHandlerOnConnCloseCtx{
			Conn:  sc,
			Error: err,
		})
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
		sxID != sc.tcpSession.id {
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
			pathAndQuery, ok := req.URL.RTSPPath()
			if !ok {
				return &base.Response{
					StatusCode: base.StatusBadRequest,
				}, liberrors.ErrServerInvalidPath{}
			}

			path, query := base.PathSplitQuery(pathAndQuery)

			res, sdp, err := h.OnDescribe(&ServerHandlerOnDescribeCtx{
				Conn:  sc,
				Req:   req,
				Path:  path,
				Query: query,
			})

			if res.StatusCode == base.StatusOK && sdp != nil {
				if res.Header == nil {
					res.Header = make(base.Header)
				}

				res.Header["Content-Base"] = base.HeaderValue{req.URL.String() + "/"}
				res.Header["Content-Type"] = base.HeaderValue{"application/sdp"}
				res.Body = sdp
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
			pathAndQuery, ok := req.URL.RTSPPath()
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
			pathAndQuery, ok := req.URL.RTSPPath()
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
				sc.tcpFrameBuffer = multibuffer.New(uint64(sc.s.ReadBufferCount), uint64(sc.s.ReadBufferSize))
			} else {
				// when playing, tcpFrameBuffer is only used to receive RTCP receiver reports,
				// that are much smaller than RTP frames and are sent at a fixed interval
				// (about 2 frames every 10 secs).
				// decrease RAM consumption by allocating less buffers.
				sc.tcpFrameBuffer = multibuffer.New(8, uint64(sc.s.ReadBufferSize))
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

			sc.tcpFrameBuffer = nil
		}

	case sc.tcpFrameEnabled: // write to background write
		sc.tcpFrameWriteBuffer.Push(res)

	default: // write directly
		sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.WriteTimeout))
		res.Write(sc.bw)
	}

	return err
}

func (sc *ServerConn) handleRequestInSession(sxID string, req *base.Request, create bool,
) (*ServerSession, *base.Response, error) {

	// if the session is already linked to this conn, communicate directly with it
	if sxID != "" {
		if ss, ok := sc.sessions[sxID]; ok {
			cres := make(chan requestRes)
			ss.request <- request{
				sc:     sc,
				req:    req,
				id:     sxID,
				create: create,
				res:    cres,
			}
			res := <-cres

			return ss, res.res, res.err
		}
	}

	// otherwise, pass through Server
	cres := make(chan requestRes)
	sc.s.sessionRequest <- request{
		sc:     sc,
		req:    req,
		id:     sxID,
		create: create,
		res:    cres,
	}
	res := <-cres

	if res.ss != nil {
		sc.sessions[res.ss.ID()] = res.ss
		sc.sessionsWG.Add(1)
	}

	return res.ss, res.res, res.err
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
