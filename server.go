package gortsplib

import (
	"fmt"
	"net"
	"time"
)

// Server is a RTSP server.
type Server struct {
	conf     ServerConf
	listener net.Listener
}

func newServer(conf ServerConf, address string) (*Server, error) {
	if conf.ReadTimeout == 0 {
		conf.ReadTimeout = 10 * time.Second
	}
	if conf.WriteTimeout == 0 {
		conf.WriteTimeout = 10 * time.Second
	}
	if conf.ReadBufferCount == 0 {
		conf.ReadBufferCount = 512
	}
	if conf.Listen == nil {
		conf.Listen = net.Listen
	}

	if conf.TLSConfig != nil && conf.UDPRTPListener != nil {
		return nil, fmt.Errorf("TLS can't be used together with UDP")
	}

	if (conf.UDPRTPListener != nil && conf.UDPRTCPListener == nil) ||
		(conf.UDPRTPListener == nil && conf.UDPRTCPListener != nil) {
		return nil, fmt.Errorf("UDPRTPListener and UDPRTPListener must be used together")
	}

	if conf.UDPRTPListener != nil {
		conf.UDPRTPListener.initialize(conf, StreamTypeRTP)
		conf.UDPRTCPListener.initialize(conf, StreamTypeRTCP)
	}

	listener, err := conf.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	s := &Server{
		conf:     conf,
		listener: listener,
	}

	return s, nil
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

	return newServerConn(s.conf, nconn), nil
}
