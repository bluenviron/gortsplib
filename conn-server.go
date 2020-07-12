package gortsplib

import (
	"bufio"
	"net"
	"time"
)

const (
	_SERVER_READ_BUFFER_SIZE  = 4096
	_SERVER_WRITE_BUFFER_SIZE = 4096
)

// ConnServerConf allows to configure a ConnServer.
type ConnServerConf struct {
	// pre-existing TCP connection that will be wrapped
	Conn net.Conn

	// (optional) timeout for read requests.
	// It defaults to 5 seconds
	ReadTimeout time.Duration

	// (optional) timeout for write requests.
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
		br:   bufio.NewReaderSize(conf.Conn, _SERVER_READ_BUFFER_SIZE),
		bw:   bufio.NewWriterSize(conf.Conn, _SERVER_WRITE_BUFFER_SIZE),
	}
}

// NetConn returns the underlying net.Conn.
func (s *ConnServer) NetConn() net.Conn {
	return s.conf.Conn
}

// ReadRequest reads a Request.
func (s *ConnServer) ReadRequest() (*Request, error) {
	s.conf.Conn.SetReadDeadline(time.Time{}) // disable deadline
	return readRequest(s.br)
}

// ReadFrameOrRequest reads an InterleavedFrame or a Request.
func (s *ConnServer) ReadFrameOrRequest(frame *InterleavedFrame) (interface{}, error) {
	s.conf.Conn.SetReadDeadline(time.Time{}) // disable deadline
	b, err := s.br.ReadByte()
	if err != nil {
		return nil, err
	}
	s.br.UnreadByte()

	if b == _INTERLEAVED_FRAME_MAGIC {
		err := frame.read(s.br)
		if err != nil {
			return nil, err
		}
		return frame, err
	}

	return readRequest(s.br)
}

// WriteResponse writes a response.
func (s *ConnServer) WriteResponse(res *Response) error {
	s.conf.Conn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return res.write(s.bw)
}

// WriteFrame writes an InterleavedFrame.
func (s *ConnServer) WriteFrame(frame *InterleavedFrame) error {
	s.conf.Conn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return frame.write(s.bw)
}
