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
	// pre-existing TCP connection to wrap
	Conn net.Conn

	// (optional) timeout of read operations.
	// It defaults to 5 seconds
	ReadTimeout time.Duration

	// (optional) timeout of write operations.
	// It defaults to 5 seconds
	WriteTimeout time.Duration

	// (optional) read buffer count.
	// If greater than 1, allows to pass frames to other routines than the one
	// that is reading frames.
	// It defaults to 1
	ReadBufferCount int
}

// ConnServer is a server-side RTSP connection.
type ConnServer struct {
	conf      ConnServerConf
	br        *bufio.Reader
	bw        *bufio.Writer
	tcpFrames *multiFrame
}

// NewConnServer allocates a ConnServer.
func NewConnServer(conf ConnServerConf) *ConnServer {
	if conf.ReadTimeout == time.Duration(0) {
		conf.ReadTimeout = 5 * time.Second
	}
	if conf.WriteTimeout == time.Duration(0) {
		conf.WriteTimeout = 5 * time.Second
	}
	if conf.ReadBufferCount == 0 {
		conf.ReadBufferCount = 1
	}

	return &ConnServer{
		conf:      conf,
		br:        bufio.NewReaderSize(conf.Conn, serverReadBufferSize),
		bw:        bufio.NewWriterSize(conf.Conn, serverWriteBufferSize),
		tcpFrames: newMultiFrame(conf.ReadBufferCount, clientTCPFrameReadBufferSize),
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
func (s *ConnServer) ReadFrameOrRequest(timeout bool) (interface{}, error) {
	if timeout {
		s.conf.Conn.SetReadDeadline(time.Now().Add(s.conf.ReadTimeout))
	}

	b, err := s.br.ReadByte()
	if err != nil {
		return nil, err
	}
	s.br.UnreadByte()

	if b == interleavedFrameMagicByte {
		frame := s.tcpFrames.next()
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

// WriteFrameTCP writes an InterleavedFrame.
func (s *ConnServer) WriteFrameTCP(frame *InterleavedFrame) error {
	s.conf.Conn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return frame.Write(s.bw)
}
