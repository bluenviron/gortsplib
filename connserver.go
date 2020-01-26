package gortsplib

import (
	"bufio"
	"net"
)

type ConnServer struct {
	nconn net.Conn
	br    *bufio.Reader
	bw    *bufio.Writer
}

func NewConnServer(nconn net.Conn) *ConnServer {
	return &ConnServer{
		nconn: nconn,
		br:    bufio.NewReaderSize(nconn, 4096),
		bw:    bufio.NewWriterSize(nconn, 4096),
	}
}

func (s *ConnServer) NetConn() net.Conn {
	return s.nconn
}

func (s *ConnServer) ReadRequest() (*Request, error) {
	return readRequest(s.br)
}

func (s *ConnServer) WriteResponse(res *Response) error {
	return res.write(s.bw)
}

func (s *ConnServer) ReadInterleavedFrame() (*InterleavedFrame, error) {
	return readInterleavedFrame(s.br)
}

func (s *ConnServer) WriteInterleavedFrame(frame *InterleavedFrame) error {
	return frame.write(s.bw)
}
