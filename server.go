package gortsplib

import (
	"bufio"
	"crypto/tls"
	"net"
)

// Server is a RTSP server.
type Server struct {
	conf     ServerConf
	listener net.Listener
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

	conn := nconn
	if s.conf.TLSConfig != nil {
		conn = tls.Server(conn, s.conf.TLSConfig)
	}

	return &ServerConn{
		s:     s,
		nconn: nconn,
		br:    bufio.NewReaderSize(conn, serverReadBufferSize),
		bw:    bufio.NewWriterSize(conn, serverWriteBufferSize),
	}, nil
}
