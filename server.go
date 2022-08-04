package gortsplib

import (
	"context"
	"crypto/rand"
	"crypto/tls"
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

		u := uint32(b[3])<<24 | uint32(b[2])<<16 | uint32(b[1])<<8 | uint32(b[0])
		id := strconv.FormatUint(uint64(u), 10)

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
	// RTSP parameters (all optional except RTSPAddress)
	//
	// the RTSP address of the server, to accept connections and send and receive
	// packets with the TCP transport.
	RTSPAddress string
	// a port to send and receive RTP packets with the UDP transport.
	// If UDPRTPAddress and UDPRTCPAddress are filled, the server can support the UDP transport.
	UDPRTPAddress string
	// a port to send and receive RTCP packets with the UDP transport.
	// If UDPRTPAddress and UDPRTCPAddress are filled, the server can support the UDP transport.
	UDPRTCPAddress string
	// a range of multicast IPs to use with the UDP-multicast transport.
	// If MulticastIPRange, MulticastRTPPort, MulticastRTCPPort are filled, the server
	// can support the UDP-multicast transport.
	MulticastIPRange string
	// a port to send RTP packets with the UDP-multicast transport.
	// If MulticastIPRange, MulticastRTPPort, MulticastRTCPPort are filled, the server
	// can support the UDP-multicast transport.
	MulticastRTPPort int
	// a port to send RTCP packets with the UDP-multicast transport.
	// If MulticastIPRange, MulticastRTPPort, MulticastRTCPPort are filled, the server
	// can support the UDP-multicast transport.
	MulticastRTCPPort int
	// timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration
	// timeout of write operations.
	// It defaults to 10 seconds
	WriteTimeout time.Duration
	// a TLS configuration to accept TLS (RTSPS) connections.
	TLSConfig *tls.Config
	// read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
	// that is reading frames.
	// It also allows to buffer routed frames and mitigate network fluctuations
	// that are particularly relevant when using UDP.
	// It defaults to 256.
	ReadBufferCount int
	// write buffer count.
	// It allows to queue packets before sending them.
	// It defaults to 256.
	WriteBufferCount int

	//
	// handler (optional)
	//
	// an handler to handle server events.
	Handler ServerHandler

	//
	// system functions (all optional)
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

	udpReceiverReportPeriod time.Duration
	udpSenderReportPeriod   time.Duration
	sessionTimeout          time.Duration
	checkStreamPeriod       time.Duration

	ctx                context.Context
	ctxCancel          func()
	wg                 sync.WaitGroup
	multicastNet       *net.IPNet
	multicastNextIP    net.IP
	tcpListener        net.Listener
	udpRTPListener     *serverUDPListener
	udpRTCPListener    *serverUDPListener
	udpRTPPacketBuffer *rtpPacketMultiBuffer
	sessions           map[string]*ServerSession
	conns              map[*ServerConn]struct{}
	closeError         error

	// in
	connClose         chan *ServerConn
	sessionRequest    chan sessionRequestReq
	sessionClose      chan *ServerSession
	streamMulticastIP chan streamMulticastIPReq
}

// Start starts the server.
func (s *Server) Start() error {
	// RTSP parameters
	if s.ReadTimeout == 0 {
		s.ReadTimeout = 10 * time.Second
	}
	if s.WriteTimeout == 0 {
		s.WriteTimeout = 10 * time.Second
	}
	if s.ReadBufferCount == 0 {
		s.ReadBufferCount = 256
	}
	if s.WriteBufferCount == 0 {
		s.WriteBufferCount = 256
	}
	if (s.WriteBufferCount & (s.WriteBufferCount - 1)) != 0 {
		return fmt.Errorf("WriteBufferCount must be a power of two")
	}

	// system functions
	if s.Listen == nil {
		s.Listen = net.Listen
	}
	if s.ListenPacket == nil {
		s.ListenPacket = net.ListenPacket
	}

	// private
	if s.udpReceiverReportPeriod == 0 {
		s.udpReceiverReportPeriod = 10 * time.Second
	}
	if s.udpSenderReportPeriod == 0 {
		s.udpSenderReportPeriod = 10 * time.Second
	}
	if s.sessionTimeout == 0 {
		s.sessionTimeout = 1 * 60 * time.Second
	}
	if s.checkStreamPeriod == 0 {
		s.checkStreamPeriod = 1 * time.Second
	}

	if s.TLSConfig != nil && s.UDPRTPAddress != "" {
		return fmt.Errorf("TLS can't be used with UDP")
	}

	if s.TLSConfig != nil && s.MulticastIPRange != "" {
		return fmt.Errorf("TLS can't be used with UDP-multicast")
	}

	if s.RTSPAddress == "" {
		return fmt.Errorf("RTSPAddress not provided")
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

		s.udpRTPListener, err = newServerUDPListener(s, false, s.UDPRTPAddress, true)
		if err != nil {
			return err
		}

		s.udpRTCPListener, err = newServerUDPListener(s, false, s.UDPRTCPAddress, false)
		if err != nil {
			s.udpRTPListener.close()
			return err
		}

		s.udpRTPPacketBuffer = newRTPPacketMultiBuffer(uint64(s.ReadBufferCount))
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
		s.udpRTPPacketBuffer = newRTPPacketMultiBuffer(uint64(s.ReadBufferCount))
	}

	var err error
	s.tcpListener, err = s.Listen("tcp", s.RTSPAddress)
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

// Close closes all the server resources and waits for them to close.
func (s *Server) Close() error {
	s.ctxCancel()
	s.wg.Wait()
	return s.closeError
}

// Wait waits until all server resources are closed.
// This can happen when a fatal error occurs or when Close() is called.
func (s *Server) Wait() error {
	s.wg.Wait()
	return s.closeError
}

func (s *Server) run() {
	defer s.wg.Done()

	s.sessions = make(map[string]*ServerSession)
	s.conns = make(map[*ServerConn]struct{})
	s.connClose = make(chan *ServerConn)
	s.sessionRequest = make(chan sessionRequestReq)
	s.sessionClose = make(chan *ServerSession)
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

	s.closeError = func() error {
		for {
			select {
			case err := <-acceptErr:
				return err

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
					if !req.sc.ip().Equal(ss.author.ip()) ||
						req.sc.zone() != ss.author.zone() {
						req.res <- sessionRequestRes{
							res: &base.Response{
								StatusCode: base.StatusBadRequest,
							},
							err: liberrors.ErrServerCannotUseSessionCreatedByOtherIP{},
						}
						continue
					}

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
				} else {
					if !req.create {
						req.res <- sessionRequestRes{
							res: &base.Response{
								StatusCode: base.StatusSessionNotFound,
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

			case req := <-s.streamMulticastIP:
				ip32 := uint32(s.multicastNextIP[0])<<24 | uint32(s.multicastNextIP[1])<<16 |
					uint32(s.multicastNextIP[2])<<8 | uint32(s.multicastNextIP[3])
				mask := uint32(s.multicastNet.Mask[0])<<24 | uint32(s.multicastNet.Mask[1])<<16 |
					uint32(s.multicastNet.Mask[2])<<8 | uint32(s.multicastNet.Mask[3])
				ip32 = (ip32 & mask) | ((ip32 + 1) & ^mask)
				ip := make(net.IP, 4)
				ip[0] = byte(ip32 >> 24)
				ip[1] = byte(ip32 >> 16)
				ip[2] = byte(ip32 >> 8)
				ip[3] = byte(ip32)
				s.multicastNextIP = ip
				req.res <- ip

			case <-s.ctx.Done():
				return liberrors.ErrServerTerminated{}
			}
		}
	}()

	s.ctxCancel()

	if s.udpRTCPListener != nil {
		s.udpRTCPListener.close()
	}

	if s.udpRTPListener != nil {
		s.udpRTPListener.close()
	}

	s.tcpListener.Close()
}

// StartAndWait starts the server and waits until a fatal error.
func (s *Server) StartAndWait() error {
	err := s.Start()
	if err != nil {
		return err
	}

	return s.Wait()
}
