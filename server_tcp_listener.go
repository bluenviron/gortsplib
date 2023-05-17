package gortsplib

import (
	"net"
)

type serverTCPListener struct {
	s  *Server
	ln net.Listener
}

func newServerTCPListener(
	s *Server,
) (*serverTCPListener, error) {
	ln, err := s.Listen(restrictNetwork("tcp", s.RTSPAddress))
	if err != nil {
		return nil, err
	}

	sl := &serverTCPListener{
		s:  s,
		ln: ln,
	}

	s.wg.Add(1)
	go sl.run()

	return sl, nil
}

func (sl *serverTCPListener) close() {
	sl.ln.Close()
}

func (sl *serverTCPListener) run() {
	defer sl.s.wg.Done()

	for {
		nconn, err := sl.ln.Accept()
		if err != nil {
			sl.s.acceptErr(err)
			return
		}

		sl.s.newConn(nconn)
	}
}
