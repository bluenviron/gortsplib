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

func newSessionSecretID(sessions map[string]*ServerSession) (string, error) {
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

type sessionRequestRes struct {
	ss  *ServerSession
	res *base.Response
	err error
}

type sessionRequestReq struct {
	sc     *ServerConn
	req    *base.Request
	id     string
	create bool
	res    chan sessionRequestRes
}

type streamMulticastIPReq struct {
	res chan net.IP
}

// Server is a RTSP server.
type Server struct {
	//
	// handler
	//
	// an handler interface to handle requests.
	Handler ServerHandler

	//
	// RTSP parameters
	//
	// timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration
	// timeout of write operations.
	// It defaults to 10 seconds
	WriteTimeout time.Duration
	// a TLS configuration to accept TLS (RTSPS) connections.
	TLSConfig *tls.Config
	// a port to send and receive RTP packets with UDP.
	// If UDPRTPAddress and UDPRTCPAddress are filled, the server can read and write UDP streams.
	UDPRTPAddress string
	// a port to send and receive RTCP packets with UDP.
	// If UDPRTPAddress and UDPRTCPAddress are filled, the server can read and write UDP streams.
	UDPRTCPAddress string
	// a range of multicast IPs to use.
	// If MulticastIPRange, MulticastRTPPort, MulticastRTCPPort are filled, the server can read and write UDP-multicast streams.
	MulticastIPRange string
	// a port to send RTP packets with UDP-multicast.
	// If MulticastIPRange, MulticastRTPPort, MulticastRTCPPort are filled, the server can read and write UDP-multicast streams.
	MulticastRTPPort int
	// a port to send RTCP packets with UDP-multicast.
	// If MulticastIPRange, MulticastRTPPort, MulticastRTCPPort are filled, the server can read and write UDP-multicast streams.
	MulticastRTCPPort int
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
	multicastNet    *net.IPNet
	multicastNextIP net.IP
	tcpListener     net.Listener
	udpRTPListener  *serverUDPListener
	udpRTCPListener *serverUDPListener
	sessions        map[string]*ServerSession
	conns           map[*ServerConn]struct{}
	exitError       error
	streams         map[*ServerStream]struct{}

	// in
	connClose         chan *ServerConn
	sessionRequest    chan sessionRequestReq
	sessionClose      chan *ServerSession
	streamAdd         chan *ServerStream
	streamRemove      chan *ServerStream
	streamMulticastIP chan streamMulticastIPReq
}

// Start starts listening on the given address.
func (s *Server) Start(address string) error {
	// RTSP parameters
	if s.ReadTimeout == 0 {
		s.ReadTimeout = 10 * time.Second
	}
	if s.WriteTimeout == 0 {
		s.WriteTimeout = 10 * time.Second
	}
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
			return fmt.Errorf("RTP and RTCP ports must be consecutive")
		}

		s.udpRTPListener, err = newServerUDPListener(s, false, s.UDPRTPAddress, StreamTypeRTP)
		if err != nil {
			return err
		}

		s.udpRTCPListener, err = newServerUDPListener(s, false, s.UDPRTCPAddress, StreamTypeRTCP)
		if err != nil {
			s.udpRTPListener.close()
			return err
		}
	}

	if s.MulticastIPRange != "" && (s.MulticastRTPPort == 0 || s.MulticastRTCPPort == 0) ||
		(s.MulticastRTPPort != 0 && (s.MulticastRTCPPort == 0 || s.MulticastIPRange == "")) ||
		s.MulticastRTCPPort != 0 && (s.MulticastRTPPort == 0 || s.MulticastIPRange == "") {
		if s.udpRTPListener != nil {
			s.udpRTPListener.close()
		}
		if s.udpRTCPListener != nil {
			s.udpRTCPListener.close()
		}
		return fmt.Errorf("MulticastIPRange, MulticastRTPPort and MulticastRTCPPort must be used together")
	}

	if s.MulticastIPRange != "" {
		if (s.MulticastRTPPort % 2) != 0 {
			if s.udpRTPListener != nil {
				s.udpRTPListener.close()
			}
			if s.udpRTCPListener != nil {
				s.udpRTCPListener.close()
			}
			return fmt.Errorf("RTP port must be even")
		}

		if s.MulticastRTCPPort != (s.MulticastRTPPort + 1) {
			if s.udpRTPListener != nil {
				s.udpRTPListener.close()
			}
			if s.udpRTCPListener != nil {
				s.udpRTCPListener.close()
			}
			return fmt.Errorf("RTP and RTCP ports must be consecutive")
		}

		var err error
		_, s.multicastNet, err = net.ParseCIDR(s.MulticastIPRange)
		if err != nil {
			if s.udpRTPListener != nil {
				s.udpRTPListener.close()
			}
			if s.udpRTCPListener != nil {
				s.udpRTCPListener.close()
			}
			return err
		}

		s.multicastNextIP = s.multicastNet.IP
	}

	var err error
	s.tcpListener, err = s.Listen("tcp", address)
	if err != nil {
		if s.udpRTPListener != nil {
			s.udpRTPListener.close()
		}
		if s.udpRTCPListener != nil {
			s.udpRTCPListener.close()
		}
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
	s.streams = make(map[*ServerStream]struct{})
	s.connClose = make(chan *ServerConn)
	s.sessionRequest = make(chan sessionRequestReq)
	s.sessionClose = make(chan *ServerSession)
	s.streamAdd = make(chan *ServerStream)
	s.streamRemove = make(chan *ServerStream)
	s.streamMulticastIP = make(chan streamMulticastIPReq)

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
			if _, ok := s.conns[sc]; !ok {
				continue
			}
			delete(s.conns, sc)
			sc.Close()

		case req := <-s.sessionRequest:
			if ss, ok := s.sessions[req.id]; ok {
				ss.request <- req
			} else {
				if !req.create {
					req.res <- sessionRequestRes{
						res: &base.Response{
							StatusCode: base.StatusBadRequest,
						},
						err: liberrors.ErrServerSessionNotFound{},
					}
					continue
				}

				secretID, err := newSessionSecretID(s.sessions)
				if err != nil {
					req.res <- sessionRequestRes{
						res: &base.Response{
							StatusCode: base.StatusBadRequest,
						},
						err: fmt.Errorf("internal error"),
					}
					continue
				}

				ss := newServerSession(s, secretID, req.sc)
				s.sessions[secretID] = ss

				select {
				case ss.request <- req:
				case <-ss.ctx.Done():
					req.res <- sessionRequestRes{
						res: &base.Response{
							StatusCode: base.StatusBadRequest,
						},
						err: liberrors.ErrServerTerminated{},
					}
				}
			}

		case ss := <-s.sessionClose:
			if sss, ok := s.sessions[ss.secretID]; !ok || sss != ss {
				continue
			}
			delete(s.sessions, ss.secretID)
			ss.Close()

		case st := <-s.streamAdd:
			s.streams[st] = struct{}{}

		case st := <-s.streamRemove:
			delete(s.streams, st)

		case req := <-s.streamMulticastIP:
			ip32 := binary.BigEndian.Uint32(s.multicastNextIP)
			mask := binary.BigEndian.Uint32(s.multicastNet.Mask)
			ip32 = (ip32 & mask) | ((ip32 + 1) & ^mask)
			ip := make(net.IP, 4)
			binary.BigEndian.PutUint32(ip, ip32)
			s.multicastNextIP = ip
			req.res <- ip

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

	for st := range s.streams {
		st.Close()
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
