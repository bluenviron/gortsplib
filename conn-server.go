package gortsplib

import (
	"bufio"
	"net"
	"time"
)

// ConnServerConf allows to configure a ConnServer.
type ConnServerConf struct {
	// pre-existing TCP connection that will be wrapped
	NConn net.Conn

	// (optional) timeout for read requests.
	// It defaults to 5 seconds
	ReadTimeout time.Duration

	// (optional) timeout for write requests.
	// It defaults to 5 seconds
	WriteTimeout time.Duration

	// (optional) size of the read buffer.
	// It defaults to 4096 bytes
	ReadBufferSize int

	// (optional) size of the write buffer.
	// It defaults to 4096 bytes
	WriteBufferSize int
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
	if conf.ReadBufferSize == 0 {
		conf.ReadBufferSize = 4096
	}
	if conf.WriteBufferSize == 0 {
		conf.WriteBufferSize = 4096
	}

	return &ConnServer{
		conf: conf,
		br:   bufio.NewReaderSize(conf.NConn, conf.ReadBufferSize),
		bw:   bufio.NewWriterSize(conf.NConn, conf.ReadBufferSize),
	}
}

// NetConn returns the underlying net.Conn.
func (s *ConnServer) NetConn() net.Conn {
	return s.conf.NConn
}

// ReadRequest reads a Request.
func (s *ConnServer) ReadRequest() (*Request, error) {
	s.conf.NConn.SetReadDeadline(time.Time{}) // disable deadline
	return readRequest(s.br)
}

// WriteResponse writes a response.
func (s *ConnServer) WriteResponse(res *Response) error {
	s.conf.NConn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return res.write(s.bw)
}

// ReadInterleavedFrame reads an InterleavedFrame.
func (s *ConnServer) ReadInterleavedFrame() (*InterleavedFrame, error) {
	s.conf.NConn.SetReadDeadline(time.Now().Add(s.conf.ReadTimeout))
	return interleavedFrameRead(s.br)
}

// WriteInterleavedFrame writes an InterleavedFrame.
func (s *ConnServer) WriteInterleavedFrame(frame *InterleavedFrame) error {
	s.conf.NConn.SetWriteDeadline(time.Now().Add(s.conf.WriteTimeout))
	return frame.write(s.bw)
}
