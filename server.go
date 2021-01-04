package gortsplib

import (
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

	return newServerConn(s, nconn), nil
}
