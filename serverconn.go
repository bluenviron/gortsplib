package gortsplib

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/multibuffer"
)

const (
	serverReadBufferSize  = 4096
	serverWriteBufferSize = 4096
)

// server errors.
var (
	ErrServerTeardown           = errors.New("teardown")
	ErrServerContentTypeMissing = errors.New("Content-Type header is missing")
	ErrServerNoTracksDefined    = errors.New("no tracks defined")
	ErrServerMissingCseq        = errors.New("CSeq is missing")
)

// ServerConn is a server-side RTSP connection.
type ServerConn struct {
	s           *Server
	nconn       net.Conn
	br          *bufio.Reader
	bw          *bufio.Writer
	mutex       sync.Mutex
	readFrames  bool
	readTimeout bool
}

// Close closes all the connection resources.
func (sc *ServerConn) Close() error {
	return sc.nconn.Close()
}

// NetConn returns the underlying net.Conn.
func (sc *ServerConn) NetConn() net.Conn {
	return sc.nconn
}

// EnableReadFrames allows or denies receiving frames.
func (sc *ServerConn) EnableReadFrames(v bool) {
	sc.readFrames = v
}

// EnableReadTimeout sets or removes the timeout on incoming packets.
func (sc *ServerConn) EnableReadTimeout(v bool) {
	sc.readTimeout = v
}

// ServerConnReadHandlers allows to set the handlers required by ServerConn.Read.
// all fields are optional.
type ServerConnReadHandlers struct {
	// called after receiving any request.
	OnRequest func(req *base.Request)

	// called before sending any response.
	OnResponse func(res *base.Response)

	// called after receiving a OPTIONS request.
	// if nil, it is generated automatically.
	OnOptions func(req *base.Request) (*base.Response, error)

	// called after receiving a DESCRIBE request.
	OnDescribe func(req *base.Request) (*base.Response, error)

	// called after receiving an ANNOUNCE request.
	OnAnnounce func(req *base.Request, tracks Tracks) (*base.Response, error)

	// called after receiving a SETUP request.
	OnSetup func(req *base.Request, th *headers.Transport) (*base.Response, error)

	// called after receiving a PLAY request.
	OnPlay func(req *base.Request) (*base.Response, error)

	// called after receiving a RECORD request.
	OnRecord func(req *base.Request) (*base.Response, error)

	// called after receiving a PAUSE request.
	OnPause func(req *base.Request) (*base.Response, error)

	// called after receiving a GET_PARAMETER request.
	// if nil, it is generated automatically.
	OnGetParameter func(req *base.Request) (*base.Response, error)

	// called after receiving a SET_PARAMETER request.
	OnSetParameter func(req *base.Request) (*base.Response, error)

	// called after receiving a TEARDOWN request.
	// if nil, it is generated automatically.
	OnTeardown func(req *base.Request) (*base.Response, error)

	// called after receiving a Frame.
	OnFrame func(trackID int, streamType StreamType, content []byte)
}

func (sc *ServerConn) backgroundRead(handlers ServerConnReadHandlers, done chan error) {
	handleRequest := func(req *base.Request) (*base.Response, error) {
		if handlers.OnRequest != nil {
			handlers.OnRequest(req)
		}

		switch req.Method {
		case base.Options:
			if handlers.OnOptions != nil {
				return handlers.OnOptions(req)
			}

			var methods []string
			if handlers.OnDescribe != nil {
				methods = append(methods, string(base.Describe))
			}
			if handlers.OnAnnounce != nil {
				methods = append(methods, string(base.Announce))
			}
			if handlers.OnSetup != nil {
				methods = append(methods, string(base.Setup))
			}
			if handlers.OnPlay != nil {
				methods = append(methods, string(base.Play))
			}
			if handlers.OnRecord != nil {
				methods = append(methods, string(base.Record))
			}
			if handlers.OnPause != nil {
				methods = append(methods, string(base.Pause))
			}
			methods = append(methods, string(base.GetParameter))
			if handlers.OnSetParameter != nil {
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
			if handlers.OnDescribe != nil {
				return handlers.OnDescribe(req)
			}

		case base.Announce:
			if handlers.OnAnnounce != nil {
				ct, ok := req.Header["Content-Type"]
				if !ok || len(ct) != 1 {
					return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, ErrServerContentTypeMissing
				}

				if ct[0] != "application/sdp" {
					return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, fmt.Errorf("unsupported Content-Type '%s'", ct)
				}

				tracks, err := ReadTracks(req.Content)
				if err != nil {
					return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, fmt.Errorf("invalid SDP: %s", err)
				}

				if len(tracks) == 0 {
					return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, ErrServerNoTracksDefined
				}

				return handlers.OnAnnounce(req, tracks)
			}

		case base.Setup:
			if handlers.OnSetup != nil {
				th, err := headers.ReadTransport(req.Header["Transport"])
				if err != nil {
					return &base.Response{
						StatusCode: base.StatusBadRequest,
					}, fmt.Errorf("transport header: %s", err)
				}

				return handlers.OnSetup(req, th)
			}

		case base.Play:
			if handlers.OnPlay != nil {
				return handlers.OnPlay(req)
			}

		case base.Record:
			if handlers.OnRecord != nil {
				return handlers.OnRecord(req)
			}

		case base.Pause:
			if handlers.OnPause != nil {
				return handlers.OnPause(req)
			}

		case base.GetParameter:
			if handlers.OnGetParameter != nil {
				return handlers.OnGetParameter(req)
			}

			// GET_PARAMETER is used like a ping
			return &base.Response{
				StatusCode: base.StatusOK,
				Header: base.Header{
					"Content-Type": base.HeaderValue{"text/parameters"},
				},
				Content: []byte("\n"),
			}, nil

		case base.SetParameter:
			if handlers.OnSetParameter != nil {
				return handlers.OnSetParameter(req)
			}

		case base.Teardown:
			if handlers.OnTeardown != nil {
				return handlers.OnTeardown(req)
			}

			return &base.Response{
				StatusCode: base.StatusOK,
			}, ErrServerTeardown
		}

		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, fmt.Errorf("unhandled method: %v", req.Method)
	}

	handleRequestOuter := func(req *base.Request) error {
		sc.mutex.Lock()
		defer sc.mutex.Unlock()

		// check cseq
		cseq, ok := req.Header["CSeq"]
		if !ok || len(cseq) != 1 {
			sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.conf.WriteTimeout))
			base.Response{
				StatusCode: base.StatusBadRequest,
				Header:     base.Header{},
			}.Write(sc.bw)
			return ErrServerMissingCseq
		}

		res, err := handleRequest(req)

		if res.Header == nil {
			res.Header = base.Header{}
		}

		// add cseq
		res.Header["CSeq"] = cseq

		// add server
		res.Header["Server"] = base.HeaderValue{"gortsplib"}

		if handlers.OnResponse != nil {
			handlers.OnResponse(res)
		}

		sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.conf.WriteTimeout))
		res.Write(sc.bw)

		return err
	}

	var req base.Request
	var frame base.InterleavedFrame
	tcpFrameBuffer := multibuffer.New(sc.s.conf.ReadBufferCount, clientTCPFrameReadBufferSize)
	var errRet error

outer:
	for {
		if sc.readTimeout {
			sc.nconn.SetReadDeadline(time.Now().Add(sc.s.conf.ReadTimeout))
		} else {
			sc.nconn.SetReadDeadline(time.Time{})
		}

		if sc.readFrames {
			frame.Content = tcpFrameBuffer.Next()
			what, err := base.ReadInterleavedFrameOrRequest(&frame, &req, sc.br)
			if err != nil {
				errRet = err
				break outer
			}

			switch what.(type) {
			case *base.InterleavedFrame:
				handlers.OnFrame(frame.TrackID, frame.StreamType, frame.Content)

			case *base.Request:
				err := handleRequestOuter(&req)
				if err != nil {
					errRet = err
					break outer
				}
			}

		} else {
			err := req.Read(sc.br)
			if err != nil {
				errRet = err
				break outer
			}

			err = handleRequestOuter(&req)
			if err != nil {
				errRet = err
				break outer
			}
		}
	}

	done <- errRet
}

// Read starts reading requests and frames.
// it returns a channel that is written when the reading stops.
func (sc *ServerConn) Read(handlers ServerConnReadHandlers) chan error {
	// channel is buffered, since listening to it is not mandatory
	done := make(chan error, 1)

	go sc.backgroundRead(handlers, done)

	return done
}

// WriteFrame writes a frame.
func (sc *ServerConn) WriteFrame(trackID int, streamType StreamType, content []byte) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.conf.WriteTimeout))
	frame := base.InterleavedFrame{
		TrackID:    trackID,
		StreamType: streamType,
		Content:    content,
	}
	return frame.Write(sc.bw)
}
