package gortsplib

import (
	"crypto/tls"
	"net"
)

type serverTCPListener struct {
	s *Server

	ln net.Listener
}

func (sl *serverTCPListener) initialize() error {
	if sl.s.TLSConfig != nil && sl.s.TLSListen != nil {
		var err error
		net, addr := restrictNetwork("tcp", sl.s.RTSPAddress)
		sl.ln, err = sl.s.TLSListen(net, addr, sl.s.TLSConfig)
		if err != nil {
			return err
		}
	} else {
		var err error
		sl.ln, err = sl.s.Listen(restrictNetwork("tcp", sl.s.RTSPAddress))
		if err != nil {
			return err
		}

		if sl.s.TLSConfig != nil {
			sl.ln = tls.NewListener(sl.ln, sl.s.TLSConfig)
		}
	}

	sl.s.wg.Add(1)
	go sl.run()

	return nil
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
