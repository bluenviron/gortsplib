package gortsplib

import (
	"bufio"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/multibuffer"
)

const (
	serverReadBufferSize  = 4096
	serverWriteBufferSize = 4096
)

// ServerConnHandler is the interface that must be implemented to use a ServerConn.
type ServerConnHandler interface {
	OnClose(err error)
	OnRequest(req *base.Request) (*base.Response, error)
	OnFrame(rackID int, streamType StreamType, content []byte)
}

// ServerConn is a server-side RTSP connection.
type ServerConn struct {
	s           *Server
	nconn       net.Conn
	connHandler ServerConnHandler
	br          *bufio.Reader
	bw          *bufio.Writer
	mutex       sync.Mutex
	frames      bool
	readTimeout bool
}

// Close closes all the ServerConn resources.
func (sc *ServerConn) Close() error {
	return sc.nconn.Close()
}

// NetConn returns the underlying net.Conn.
func (sc *ServerConn) NetConn() net.Conn {
	return sc.nconn
}

// EnableFrames allows or denies receiving frames.
func (sc *ServerConn) EnableFrames(v bool) {
	sc.frames = v
}

// EnableReadTimeout sets or removes the timeout on incoming packets.
func (sc *ServerConn) EnableReadTimeout(v bool) {
	sc.readTimeout = v
}

func (sc *ServerConn) run() {
	defer sc.s.wg.Done()

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

		if sc.frames {
			frame.Content = tcpFrameBuffer.Next()
			what, err := base.ReadInterleavedFrameOrRequest(&frame, &req, sc.br)
			if err != nil {
				errRet = err
				break outer
			}

			switch what.(type) {
			case *base.InterleavedFrame:
				sc.connHandler.OnFrame(frame.TrackID, frame.StreamType, frame.Content)

			case *base.Request:
				err := sc.handleRequest(&req)
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

			err = sc.handleRequest(&req)
			if err != nil {
				errRet = err
				break outer
			}
		}
	}

	sc.nconn.Close()
	sc.connHandler.OnClose(errRet)
}

func (sc *ServerConn) handleRequest(req *base.Request) error {
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
		return fmt.Errorf("cseq is missing")
	}

	res, err := sc.connHandler.OnRequest(req)

	// add cseq to response
	res.Header["CSeq"] = cseq

	sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.conf.WriteTimeout))
	res.Write(sc.bw)

	return err
}

// WriteFrame writes a frame.
func (sc *ServerConn) WriteFrame(trackID int, streamType StreamType, content []byte) error {
	sc.mutex.Lock()
	defer sc.mutex.Unlock()

	frame := base.InterleavedFrame{
		TrackID:    trackID,
		StreamType: streamType,
		Content:    content,
	}

	sc.nconn.SetWriteDeadline(time.Now().Add(sc.s.conf.WriteTimeout))
	return frame.Write(sc.bw)
}
