package gortsplib

import (
	"bufio"
	"net"
)

// Server is a RTSP server.
type Server struct {
	conf     ServerConf
	listener *net.TCPListener
}

// Close closes the server.
func (s *Server) Close() error {
	return s.listener.Close()
}

// Accept accepts a connection.
func (s *Server) Accept() (*ServerConn, error) {
	nconn, err := s.listener.Accept()
	if err != nil {
		return nil, err
	}

	return &ServerConn{
		s:     s,
		nconn: nconn,
		br:    bufio.NewReaderSize(nconn, serverReadBufferSize),
		bw:    bufio.NewWriterSize(nconn, serverWriteBufferSize),
	}, nil
}
