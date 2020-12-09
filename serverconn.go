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

func (sc *ServerConn) backgroundRead(
	onRequest func(req *base.Request) (*base.Response, error),
	onFrame func(trackID int, streamType StreamType, content []byte),
	done chan error,
) {
	handleRequest := func(req *base.Request) error {
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

		res, err := onRequest(req)

		// add cseq to response
		if res.Header == nil {
			res.Header = base.Header{}
		}
		res.Header["CSeq"] = cseq

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
				onFrame(frame.TrackID, frame.StreamType, frame.Content)

			case *base.Request:
				err := handleRequest(&req)
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

			err = handleRequest(&req)
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
func (sc *ServerConn) Read(
	onRequest func(req *base.Request) (*base.Response, error),
	onFrame func(trackID int, streamType StreamType, content []byte),
) chan error {
	// channel is buffered, since listening to it is not mandatory
	done := make(chan error, 1)

	go sc.backgroundRead(onRequest, onFrame, done)

	return done
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
