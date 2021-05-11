package gortsplib

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/liberrors"
)

func extractPort(address string) (int, error) {
	_, tmp, err := net.SplitHostPort(address)
	if err != nil {
		return 0, err
	}

	tmp2, err := strconv.ParseInt(tmp, 10, 64)
	if err != nil {
		return 0, err
	}

	return int(tmp2), nil
}

func newSessionID(sessions map[string]*ServerSession) (string, error) {
	for {
		b := make([]byte, 4)
		_, err := rand.Read(b)
		if err != nil {
			return "", err
		}

		id := strconv.FormatUint(uint64(binary.LittleEndian.Uint32(b)), 10)

		if _, ok := sessions[id]; !ok {
			return id, nil
		}
	}
}

type requestRes struct {
	ss  *ServerSession
	res *base.Response
	err error
}

type request struct {
	sc     *ServerConn
	req    *base.Request
	id     string
	create bool
	res    chan requestRes
}

// Server is a RTSP server.
type Server struct {
	//
	// handler
	//
	// an handler interface to handle requests.
	Handler ServerHandler

	//
	// connection
	//
	// timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration
	// timeout of write operations.
	// It defaults to 10 seconds
	WriteTimeout time.Duration
	// a TLS configuration to accept TLS (RTSPS) connections.
	TLSConfig *tls.Config
	// a port to send and receive UDP/RTP packets.
	// If UDPRTPAddress and UDPRTCPAddress are != "", the server can accept and send UDP streams.
	UDPRTPAddress string
	// a port to send and receive UDP/RTCP packets.
	// If UDPRTPAddress and UDPRTCPAddress are != "", the server can accept and send UDP streams.
	UDPRTCPAddress string

	//
	// reading / writing
	//
	// read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
	// that is reading frames.
	// It also allows to buffer routed frames and mitigate network fluctuations
	// that are particularly high when using UDP.
	// It defaults to 512
	ReadBufferCount int
	// read buffer size.
	// This must be touched only when the server reports problems about buffer sizes.
	// It defaults to 2048.
	ReadBufferSize int

	//
	// system functions
	//
	// function used to initialize the TCP listener.
	// It defaults to net.Listen.
	Listen func(network string, address string) (net.Listener, error)
	// function used to initialize UDP listeners.
	// It defaults to net.ListenPacket.
	ListenPacket func(network, address string) (net.PacketConn, error)

	//
	// private
	//

	receiverReportPeriod           time.Duration
	closeSessionAfterNoRequestsFor time.Duration

	ctx             context.Context
	ctxCancel       func()
	wg              sync.WaitGroup
	tcpListener     net.Listener
	udpRTPListener  *serverUDPListener
	udpRTCPListener *serverUDPListener
	sessions        map[string]*ServerSession
	conns           map[*ServerConn]struct{}
	exitError       error

	// in
	connClose      chan *ServerConn
	sessionRequest chan request
	sessionClose   chan *ServerSession
}

// Start starts listening on the given address.
func (s *Server) Start(address string) error {
	// connection
	if s.ReadTimeout == 0 {
		s.ReadTimeout = 10 * time.Second
	}
	if s.WriteTimeout == 0 {
		s.WriteTimeout = 10 * time.Second
	}

	// reading / writing
	if s.ReadBufferCount == 0 {
		s.ReadBufferCount = 512
	}
	if s.ReadBufferSize == 0 {
		s.ReadBufferSize = 2048
	}

	// system functions
	if s.Listen == nil {
		s.Listen = net.Listen
	}
	if s.ListenPacket == nil {
		s.ListenPacket = net.ListenPacket
	}

	// private
	if s.receiverReportPeriod == 0 {
		s.receiverReportPeriod = 10 * time.Second
	}
	if s.closeSessionAfterNoRequestsFor == 0 {
		s.closeSessionAfterNoRequestsFor = 1 * 60 * time.Second
	}

	if s.TLSConfig != nil && s.UDPRTPAddress != "" {
		return fmt.Errorf("TLS can't be used together with UDP")
	}

	if (s.UDPRTPAddress != "" && s.UDPRTCPAddress == "") ||
		(s.UDPRTPAddress == "" && s.UDPRTCPAddress != "") {
		return fmt.Errorf("UDPRTPAddress and UDPRTCPAddress must be used together")
	}

	if s.UDPRTPAddress != "" {
		rtpPort, err := extractPort(s.UDPRTPAddress)
		if err != nil {
			return err
		}

		rtcpPort, err := extractPort(s.UDPRTCPAddress)
		if err != nil {
			return err
		}

		if (rtpPort % 2) != 0 {
			return fmt.Errorf("RTP port must be even")
		}

		if rtcpPort != (rtpPort + 1) {
			return fmt.Errorf("RTCP and RTP ports must be consecutive")
		}

		s.udpRTPListener, err = newServerUDPListener(s, s.UDPRTPAddress, StreamTypeRTP)
		if err != nil {
			return err
		}

		s.udpRTCPListener, err = newServerUDPListener(s, s.UDPRTCPAddress, StreamTypeRTCP)
		if err != nil {
			s.udpRTPListener.close()
			return err
		}
	}

	var err error
	s.tcpListener, err = s.Listen("tcp", address)
	if err != nil {
		s.udpRTPListener.close()
		s.udpRTPListener.close()
		return err
	}

	s.ctx, s.ctxCancel = context.WithCancel(context.Background())

	s.wg.Add(1)
	go s.run()

	return nil
}

// Close closes the server and waits for all its resources to exit.
func (s *Server) Close() error {
	s.ctxCancel()
	s.wg.Wait()
	return nil
}

// Wait waits until a fatal error.
func (s *Server) Wait() error {
	s.wg.Wait()
	return s.exitError
}

func (s *Server) run() {
	defer s.wg.Done()

	s.sessions = make(map[string]*ServerSession)
	s.conns = make(map[*ServerConn]struct{})
	s.connClose = make(chan *ServerConn)
	s.sessionRequest = make(chan request)
	s.sessionClose = make(chan *ServerSession)

	s.wg.Add(1)
	connNew := make(chan net.Conn)
	acceptErr := make(chan error)
	go func() {
		defer s.wg.Done()
		err := func() error {
			for {
				nconn, err := s.tcpListener.Accept()
				if err != nil {
					return err
				}

				select {
				case connNew <- nconn:
				case <-s.ctx.Done():
					nconn.Close()
				}
			}
		}()

		select {
		case acceptErr <- err:
		case <-s.ctx.Done():
		}
	}()

outer:
	for {
		select {
		case err := <-acceptErr:
			s.exitError = err
			break outer

		case nconn := <-connNew:
			sc := newServerConn(s, nconn)
			s.conns[sc] = struct{}{}

		case sc := <-s.connClose:
			s.doConnClose(sc)

		case req := <-s.sessionRequest:
			if ss, ok := s.sessions[req.id]; ok {
				ss.request <- req

			} else {
				if !req.create {
					req.res <- requestRes{
						res: &base.Response{
							StatusCode: base.StatusBadRequest,
						},
						err: liberrors.ErrServerSessionNotFound{},
					}
					continue
				}

				id, err := newSessionID(s.sessions)
				if err != nil {
					req.res <- requestRes{
						res: &base.Response{
							StatusCode: base.StatusBadRequest,
						},
						err: fmt.Errorf("internal error"),
					}
					continue
				}

				ss := newServerSession(s, id, req.sc)
				s.sessions[id] = ss

				select {
				case ss.request <- req:
				case <-ss.ctx.Done():
					req.res <- requestRes{
						res: &base.Response{
							StatusCode: base.StatusBadRequest,
						},
						err: liberrors.ErrServerTerminated{},
					}
				}
			}

		case ss := <-s.sessionClose:
			if sss, ok := s.sessions[ss.id]; !ok || sss != ss {
				continue
			}
			s.doSessionClose(ss)

		case <-s.ctx.Done():
			break outer
		}
	}

	s.ctxCancel()

	if s.udpRTCPListener != nil {
		s.udpRTCPListener.close()
	}

	if s.udpRTPListener != nil {
		s.udpRTPListener.close()
	}

	s.tcpListener.Close()

	for sc := range s.conns {
		s.doConnClose(sc)
	}

	for _, ss := range s.sessions {
		s.doSessionClose(ss)
	}
}

// StartAndWait starts the server and waits until a fatal error.
func (s *Server) StartAndWait(address string) error {
	err := s.Start(address)
	if err != nil {
		return err
	}

	return s.Wait()
}

func (s *Server) doConnClose(sc *ServerConn) {
	delete(s.conns, sc)
	sc.Close()
}

func (s *Server) doSessionClose(ss *ServerSession) {
	delete(s.sessions, ss.id)
	ss.Close()
}
