package gortsplib

import (
	"bufio"
	"net"
)

type ConnServer struct {
	nconn net.Conn
	bw    *bufio.Writer
}

func NewConnServer(nconn net.Conn) *ConnServer {
	return &ConnServer{
		nconn: nconn,
		bw:    bufio.NewWriterSize(nconn, _INTERLEAVED_FRAME_MAX_SIZE),
	}
}

func (s *ConnServer) NetConn() net.Conn {
	return s.nconn
}

func (s *ConnServer) ReadRequest() (*Request, error) {
	return readRequest(s.nconn)
}

func (s *ConnServer) WriteResponse(res *Response) error {
	return res.write(s.nconn)
}

func (s *ConnServer) ReadInterleavedFrame() (*InterleavedFrame, error) {
	return readInterleavedFrame(s.nconn)
}

func (s *ConnServer) WriteInterleavedFrame(frame *InterleavedFrame) error {
	return frame.write(s.bw)
}
