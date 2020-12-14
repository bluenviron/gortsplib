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

	conn := func() net.Conn {
		if s.conf.TLSConfig != nil {
			return tls.Server(nconn, s.conf.TLSConfig)
		}
		return nconn
	}()

	return &ServerConn{
		s:     s,
		nconn: nconn,
		br:    bufio.NewReaderSize(conn, serverReadBufferSize),
		bw:    bufio.NewWriterSize(conn, serverWriteBufferSize),
	}, nil
}
