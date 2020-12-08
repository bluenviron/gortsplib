package gortsplib

import (
	"bufio"
	"net"
	"sync"
	"time"
)

// Server is a RTSP server.
type Server struct {
	conf     ServerConf
	listener *net.TCPListener
	handler  func(sc *ServerConn) ServerConnHandler
	wg       sync.WaitGroup
}

// Close closes the server.
func (s *Server) Close() error {
	s.listener.Close()
	s.wg.Wait()
	return nil
}

func (s *Server) run() {
	defer s.wg.Done()

	if s.conf.ReadTimeout == 0 {
		s.conf.ReadTimeout = 10 * time.Second
	}
	if s.conf.WriteTimeout == 0 {
		s.conf.WriteTimeout = 10 * time.Second
	}
	if s.conf.ReadBufferCount == 0 {
		s.conf.ReadBufferCount = 1
	}

	for {
		nconn, err := s.listener.Accept()
		if err != nil {
			break
		}

		sc := &ServerConn{
			s:     s,
			nconn: nconn,
			br:    bufio.NewReaderSize(nconn, serverReadBufferSize),
			bw:    bufio.NewWriterSize(nconn, serverWriteBufferSize),
		}

		sc.connHandler = s.handler(sc)
		if sc.connHandler == nil {
			nconn.Close()
			continue
		}

		s.wg.Add(1)
		go sc.run()
	}
}
