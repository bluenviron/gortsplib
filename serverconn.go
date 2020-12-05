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

// ServerConnConf allows to configure a ServerConn.
type ServerConnConf struct {
	// pre-existing TCP connection to wrap
	Conn net.Conn

	// (optional) timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration

	// (optional) timeout of write operations.
	// It defaults to 10 seconds
	WriteTimeout time.Duration

	// (optional) read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
	// that is reading frames.
	// It defaults to 1
	ReadBufferCount int
}

// ServerConn is a server-side RTSP connection.
type ServerConn struct {
	conf           ServerConnConf
	br             *bufio.Reader
	bw             *bufio.Writer
	request        *base.Request
	frame          *base.InterleavedFrame
	tcpFrameBuffer *multibuffer.MultiBuffer
}

// NewServerConn allocates a ServerConn.
func NewServerConn(conf ServerConnConf) *ServerConn {
	if conf.ReadTimeout == 0 {
		conf.ReadTimeout = 10 * time.Second
	}
	if conf.WriteTimeout == 0 {
		conf.WriteTimeout = 10 * time.Second
	}
	if conf.ReadBufferCount == 0 {
		conf.ReadBufferCount = 1
	}

	return &ServerConn{
		conf:           conf,
		br:             bufio.NewReaderSize(conf.Conn, serverReadBufferSize),
		bw:             bufio.NewWriterSize(conf.Conn, serverWriteBufferSize),
		request:        &base.Request{},
		frame:          &base.InterleavedFrame{},
		tcpFrameBuffer: multibuffer.New(conf.ReadBufferCount, clientTCPFrameReadBufferSize),
	}
}

// Close closes all the ServerConn resources.
func (s *ServerConn) Close() error {
	return s.conf.Conn.Close()
}

// NetConn returns the underlying net.Conn.
func (s *ServerConn) NetConn() net.Conn {
	return s.conf.Conn
}

// ReadRequest reads a Request.
func (s *ServerConn) ReadRequest() (*base.Request, error) {
	s.conf.Conn.SetReadDeadline(time.Time{}) // disable deadline
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
		s.conf.Conn.SetReadDeadline(time.Now().Add(s.conf.ReadTimeout))
	}

	return base.ReadInterleavedFrameOrRequest(s.frame, s.request, s.br)
}

// WriteResponse writes a Response.
func (s *ServerConn) WriteResponse(res *base.Response) error {
	s.conf.Conn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return res.Write(s.bw)
}

// WriteFrameTCP writes an InterleavedFrame.
func (s *ServerConn) WriteFrameTCP(trackID int, streamType StreamType, content []byte) error {
	frame := base.InterleavedFrame{
		TrackID:    trackID,
		StreamType: streamType,
		Content:    content,
	}

	s.conf.Conn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return frame.Write(s.bw)
}
