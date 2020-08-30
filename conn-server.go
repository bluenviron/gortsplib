package gortsplib

import (
	"bufio"
	"net"
	"time"
)

const (
	serverReadBufferSize  = 4096
	serverWriteBufferSize = 4096
)

// ConnServerConf allows to configure a ConnServer.
type ConnServerConf struct {
	// pre-existing TCP connection that will be wrapped
	Conn net.Conn

	// (optional) timeout of read operations.
	// It defaults to 5 seconds
	ReadTimeout time.Duration

	// (optional) timeout of write operations.
	// It defaults to 5 seconds
	WriteTimeout time.Duration
}

// ConnServer is a server-side RTSP connection.
type ConnServer struct {
	conf ConnServerConf
	br   *bufio.Reader
	bw   *bufio.Writer
}

// NewConnServer allocates a ConnClient.
func NewConnServer(conf ConnServerConf) *ConnServer {
	if conf.ReadTimeout == time.Duration(0) {
		conf.ReadTimeout = 5 * time.Second
	}
	if conf.WriteTimeout == time.Duration(0) {
		conf.WriteTimeout = 5 * time.Second
	}

	return &ConnServer{
		conf: conf,
		br:   bufio.NewReaderSize(conf.Conn, serverReadBufferSize),
		bw:   bufio.NewWriterSize(conf.Conn, serverWriteBufferSize),
	}
}

// Close closes all the ConnServer resources.
func (s *ConnServer) Close() error {
	return s.conf.Conn.Close()
}

// NetConn returns the underlying net.Conn.
func (s *ConnServer) NetConn() net.Conn {
	return s.conf.Conn
}

// ReadRequest reads a Request.
func (s *ConnServer) ReadRequest() (*Request, error) {
	s.conf.Conn.SetReadDeadline(time.Time{}) // disable deadline
	return ReadRequest(s.br)
}

// ReadFrameOrRequest reads an InterleavedFrame or a Request.
func (s *ConnServer) ReadFrameOrRequest(frame *InterleavedFrame, timeout bool) (interface{}, error) {
	if timeout {
		s.conf.Conn.SetReadDeadline(time.Now().Add(s.conf.ReadTimeout))
	}

	b, err := s.br.ReadByte()
	if err != nil {
		return nil, err
	}
	s.br.UnreadByte()

	if b == interleavedFrameMagicByte {
		err := frame.Read(s.br)
		if err != nil {
			return nil, err
		}
		return frame, err
	}

	return ReadRequest(s.br)
}

// WriteResponse writes a Response.
func (s *ConnServer) WriteResponse(res *Response) error {
	s.conf.Conn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return res.Write(s.bw)
}

// WriteFrame writes an InterleavedFrame.
func (s *ConnServer) WriteFrame(frame *InterleavedFrame) error {
	s.conf.Conn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return frame.Write(s.bw)
}
