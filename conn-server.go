package gortsplib

import (
	"bufio"
	"net"
	"time"
)

// ConnServer is a server-side RTSP connection.
type ConnServer struct {
	nconn        net.Conn
	br           *bufio.Reader
	bw           *bufio.Writer
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// NewConnServer allocates a ConnClient.
func NewConnServer(nconn net.Conn, readTimeout time.Duration, writeTimeout time.Duration) *ConnServer {
	return &ConnServer{
		nconn:        nconn,
		br:           bufio.NewReaderSize(nconn, 4096),
		bw:           bufio.NewWriterSize(nconn, 4096),
		readTimeout:  readTimeout,
		writeTimeout: writeTimeout,
	}
}

// NetConn returns the underlying net.Conn.
func (s *ConnServer) NetConn() net.Conn {
	return s.nconn
}

// ReadRequest reads a Request.
func (s *ConnServer) ReadRequest() (*Request, error) {
	s.nconn.SetReadDeadline(time.Time{}) // disable deadline
	return readRequest(s.br)
}

// WriteResponse writes a response.
func (s *ConnServer) WriteResponse(res *Response) error {
	s.nconn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
	return res.write(s.bw)
}

// ReadInterleavedFrame reads an InterleavedFrame.
func (s *ConnServer) ReadInterleavedFrame() (*InterleavedFrame, error) {
	s.nconn.SetReadDeadline(time.Now().Add(s.readTimeout))
	return readInterleavedFrame(s.br)
}

// WriteInterleavedFrame writes an InterleavedFrame.
func (s *ConnServer) WriteInterleavedFrame(frame *InterleavedFrame) error {
	s.nconn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
	return frame.write(s.bw)
}
