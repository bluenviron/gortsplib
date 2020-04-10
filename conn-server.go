package gortsplib

import (
	"bufio"
	"net"
)

// ConnServer is a server-side RTSP connection.
type ConnServer struct {
	nconn net.Conn
	br    *bufio.Reader
	bw    *bufio.Writer
}

// NewConnServer allocates a ConnClient.
func NewConnServer(nconn net.Conn) *ConnServer {
	return &ConnServer{
		nconn: nconn,
		br:    bufio.NewReaderSize(nconn, 4096),
		bw:    bufio.NewWriterSize(nconn, 4096),
	}
}

// NetConn returns the underlying new.Conn.
func (s *ConnServer) NetConn() net.Conn {
	return s.nconn
}

// ReadRequest reads a Request.
func (s *ConnServer) ReadRequest() (*Request, error) {
	return readRequest(s.br)
}

// WriteResponse writes a response.
func (s *ConnServer) WriteResponse(res *Response) error {
	return res.write(s.bw)
}

// ReadInterleavedFrame reads an InterleavedFrame.
func (s *ConnServer) ReadInterleavedFrame() (*InterleavedFrame, error) {
	return readInterleavedFrame(s.br)
}

// WriteInterleavedFrame writes an InterleavedFrame.
func (s *ConnServer) WriteInterleavedFrame(frame *InterleavedFrame) error {
	return frame.write(s.bw)
}
