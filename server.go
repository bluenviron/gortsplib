package gortsplib

import (
	"bufio"
	"net"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/multibuffer"
)

type ServerHandler interface {
}

type Server struct {
	c        ServerConf
	listener *net.TCPListener
}

func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) Accept() (*ServerConn, error) {
	nconn, err := s.listener.Accept()
	if err != nil {
		return nil, err
	}

	if s.c.ReadTimeout == 0 {
		s.c.ReadTimeout = 10 * time.Second
	}
	if s.c.WriteTimeout == 0 {
		s.c.WriteTimeout = 10 * time.Second
	}
	if s.c.ReadBufferCount == 0 {
		s.c.ReadBufferCount = 1
	}

	sc := &ServerConn{
		c:              s.c,
		nconn:          nconn,
		br:             bufio.NewReaderSize(nconn, serverReadBufferSize),
		bw:             bufio.NewWriterSize(nconn, serverWriteBufferSize),
		request:        &base.Request{},
		frame:          &base.InterleavedFrame{},
		tcpFrameBuffer: multibuffer.New(s.c.ReadBufferCount, clientTCPFrameReadBufferSize),
	}

	return sc, nil
}
