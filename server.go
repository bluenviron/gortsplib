package gortsplib

import (
	"bufio"
	"net"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/multibuffer"
)

// ServerHandler is the interface that must be implemented to use a Server.
type ServerHandler interface {
}

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

	if s.conf.ReadTimeout == 0 {
		s.conf.ReadTimeout = 10 * time.Second
	}
	if s.conf.WriteTimeout == 0 {
		s.conf.WriteTimeout = 10 * time.Second
	}
	if s.conf.ReadBufferCount == 0 {
		s.conf.ReadBufferCount = 1
	}

	sc := &ServerConn{
		conf:           s.conf,
		nconn:          nconn,
		br:             bufio.NewReaderSize(nconn, serverReadBufferSize),
		bw:             bufio.NewWriterSize(nconn, serverWriteBufferSize),
		request:        &base.Request{},
		frame:          &base.InterleavedFrame{},
		tcpFrameBuffer: multibuffer.New(s.conf.ReadBufferCount, clientTCPFrameReadBufferSize),
	}

	return sc, nil
}
