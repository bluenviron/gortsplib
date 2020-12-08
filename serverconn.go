package gortsplib

import (
	"bufio"
	"net"
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
	conf           ServerConf
	nconn          net.Conn
	br             *bufio.Reader
	bw             *bufio.Writer
	request        *base.Request
	frame          *base.InterleavedFrame
	tcpFrameBuffer *multibuffer.MultiBuffer
}

// Close closes all the ServerConn resources.
func (s *ServerConn) Close() error {
	return s.nconn.Close()
}

// NetConn returns the underlying net.Conn.
func (s *ServerConn) NetConn() net.Conn {
	return s.nconn
}

// ReadRequest reads a Request.
func (s *ServerConn) ReadRequest() (*base.Request, error) {
	s.nconn.SetReadDeadline(time.Time{}) // disable deadline
	err := s.request.Read(s.br)
	if err != nil {
		return nil, err
	}

	return s.request, nil
}

// ReadFrameTCPOrRequest reads an InterleavedFrame or a Request.
func (s *ServerConn) ReadFrameTCPOrRequest(timeout bool) (interface{}, error) {
	s.frame.Content = s.tcpFrameBuffer.Next()

	if timeout {
		s.nconn.SetReadDeadline(time.Now().Add(s.conf.ReadTimeout))
	}

	return base.ReadInterleavedFrameOrRequest(s.frame, s.request, s.br)
}

// WriteResponse writes a Response.
func (s *ServerConn) WriteResponse(res *base.Response) error {
	s.nconn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return res.Write(s.bw)
}

// WriteFrameTCP writes an InterleavedFrame.
func (s *ServerConn) WriteFrameTCP(trackID int, streamType StreamType, content []byte) error {
	frame := base.InterleavedFrame{
		TrackID:    trackID,
		StreamType: streamType,
		Content:    content,
	}

	s.nconn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return frame.Write(s.bw)
}
