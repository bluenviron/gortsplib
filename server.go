package gortsplib

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/auth"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
)

const (
	serverHeader    = "gortsplib"
	serverAuthRealm = "ipcam"
)

var errHTTPUpgraded = errors.New("upgraded to HTTP conn")

func extractPort(address string) (int, error) {
	_, tmp, err := net.SplitHostPort(address)
	if err != nil {
		return 0, err
	}

	tmp2, err := strconv.ParseUint(tmp, 10, 16)
	if err != nil {
		return 0, err
	}

	return int(tmp2), nil
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

type sessionHandleHTTPChannelReq struct {
	sc       *ServerConn
	write    bool
	tunnelID string
	res      chan error
}

type chGetMulticastIPReq struct {
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
	// It defaults to 10 seconds.
	ReadTimeout time.Duration
	// timeout of write operations.
	// It defaults to 10 seconds.
	WriteTimeout time.Duration
	// read timeout of idle connections and sessions.
	// It defaults to 60 seconds.
	IdleTimeout time.Duration
	// a TLS configuration to accept TLS (RTSPS) connections.
	TLSConfig *tls.Config
	// Size of the UDP read buffer.
	// This can be increased to reduce packet losses.
	// It defaults to the operating system default value.
	UDPReadBufferSize int
	// Size of the queue of outgoing packets.
	// It defaults to 256.
	WriteQueueSize int
	// maximum size of outgoing RTP / RTCP packets.
	// This must be less than the UDP MTU (1472 bytes).
	// It defaults to 1472.
	MaxPacketSize int
	// disable automatic RTCP sender reports.
	DisableRTCPSenderReports bool
	// authentication methods.
	// It defaults to plain and digest+MD5.
	AuthMethods []auth.VerifyMethod

	//
	// handler (optional)
	//
	// an handler to handle server events.
	// It may implement one or more of the ServerHandler* interfaces.
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

	timeNow              func() time.Time
	senderReportPeriod   time.Duration
	receiverReportPeriod time.Duration
	checkStreamPeriod    time.Duration

	ctx              context.Context
	ctxCancel        func()
	wg               sync.WaitGroup
	multicastNet     *net.IPNet
	multicastNextIP  net.IP
	tcpListener      *serverTCPListener
	udpRTPListener   *serverUDPListener
	udpRTCPListener  *serverUDPListener
	conns            map[*ServerConn]struct{}
	httpReadChannels map[*ServerConn]chan error
	sessions         map[string]*ServerSession
	closeError       error

	// in
	chNewConn           chan net.Conn
	chAcceptErr         chan error
	chCloseConn         chan *ServerConn
	chHandleHTTPChannel chan sessionHandleHTTPChannelReq
	chHandleRequest     chan sessionRequestReq
	chCloseSession      chan *ServerSession
	chGetMulticastIP    chan chGetMulticastIPReq
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
	if s.IdleTimeout == 0 {
		s.IdleTimeout = 60 * time.Second
	}
	if s.WriteQueueSize == 0 {
		s.WriteQueueSize = 256
	} else if (s.WriteQueueSize & (s.WriteQueueSize - 1)) != 0 {
		return fmt.Errorf("WriteQueueSize (%d) must be a power of two", s.WriteQueueSize)
	}
	if s.MaxPacketSize == 0 {
		s.MaxPacketSize = udpMaxPayloadSize
	} else if s.MaxPacketSize > udpMaxPayloadSize {
		return fmt.Errorf("MaxPacketSize (%d) must be less than %d", s.MaxPacketSize, udpMaxPayloadSize)
	}
	if len(s.AuthMethods) == 0 {
		// disable VerifyMethodDigestSHA256 unless explicitly set
		// since it prevents FFmpeg from authenticating
		s.AuthMethods = []auth.VerifyMethod{auth.VerifyMethodBasic, auth.VerifyMethodDigestMD5}
	}

	// system functions
	if s.Listen == nil {
		s.Listen = net.Listen
	}
	if s.ListenPacket == nil {
		s.ListenPacket = net.ListenPacket
	}

	// private
	if s.timeNow == nil {
		s.timeNow = time.Now
	}
	if s.senderReportPeriod == 0 {
		s.senderReportPeriod = 10 * time.Second
	}
	if s.receiverReportPeriod == 0 {
		s.receiverReportPeriod = 10 * time.Second
	}
	if s.checkStreamPeriod == 0 {
		s.checkStreamPeriod = 1 * time.Second
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
			return fmt.Errorf("RTP port (%d) must be even", rtpPort)
		}

		if rtcpPort != (rtpPort + 1) {
			return fmt.Errorf("RTP (%d) and RTCP (%d) ports must be consecutive", rtpPort, rtcpPort)
		}

		s.udpRTPListener = &serverUDPListener{
			readBufferSize:  s.UDPReadBufferSize,
			listenPacket:    s.ListenPacket,
			writeTimeout:    s.WriteTimeout,
			multicastEnable: false,
			address:         s.UDPRTPAddress,
		}
		err = s.udpRTPListener.initialize()
		if err != nil {
			return err
		}

		s.udpRTCPListener = &serverUDPListener{
			readBufferSize:  s.UDPReadBufferSize,
			listenPacket:    s.ListenPacket,
			writeTimeout:    s.WriteTimeout,
			multicastEnable: false,
			address:         s.UDPRTCPAddress,
		}
		err = s.udpRTCPListener.initialize()
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
			return fmt.Errorf("RTP port (%d) must be even", s.MulticastRTPPort)
		}

		if s.MulticastRTCPPort != (s.MulticastRTPPort + 1) {
			if s.udpRTPListener != nil {
				s.udpRTPListener.close()
			}
			if s.udpRTCPListener != nil {
				s.udpRTCPListener.close()
			}
			return fmt.Errorf("multicast RTP (%d) and RTCP (%d) ports must be consecutive",
				s.MulticastRTPPort, s.MulticastRTCPPort)
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

	s.ctx, s.ctxCancel = context.WithCancel(context.Background())

	s.conns = make(map[*ServerConn]struct{})
	s.httpReadChannels = make(map[*ServerConn]chan error)
	s.sessions = make(map[string]*ServerSession)
	s.chNewConn = make(chan net.Conn)
	s.chAcceptErr = make(chan error)
	s.chCloseConn = make(chan *ServerConn)
	s.chHandleHTTPChannel = make(chan sessionHandleHTTPChannelReq)
	s.chHandleRequest = make(chan sessionRequestReq)
	s.chCloseSession = make(chan *ServerSession)
	s.chGetMulticastIP = make(chan chGetMulticastIPReq)

	s.tcpListener = &serverTCPListener{s: s}
	err := s.tcpListener.initialize()
	if err != nil {
		if s.udpRTPListener != nil {
			s.udpRTPListener.close()
		}
		if s.udpRTCPListener != nil {
			s.udpRTCPListener.close()
		}
		s.ctxCancel()
		return err
	}

	s.wg.Add(1)
	go s.run()

	return nil
}

// Close closes all the server resources and waits for them to exit.
func (s *Server) Close() {
	s.ctxCancel()
	s.wg.Wait()
}

// Wait waits until all server resources are closed.
// This can happen when a fatal error occurs or when Close() is called.
func (s *Server) Wait() error {
	s.wg.Wait()
	return s.closeError
}

func (s *Server) run() {
	defer s.wg.Done()

	s.closeError = s.runInner()

	s.ctxCancel()

	s.tcpListener.close()

	if s.udpRTCPListener != nil {
		s.udpRTCPListener.close()
	}

	if s.udpRTPListener != nil {
		s.udpRTPListener.close()
	}
}

func (s *Server) runInner() error {
	for {
		select {
		case err := <-s.chAcceptErr:
			return err

		case nconn := <-s.chNewConn:
			sc := &ServerConn{
				s:     s,
				nconn: nconn,
			}
			sc.initialize()
			s.conns[sc] = struct{}{}

		case sc := <-s.chCloseConn:
			if _, ok := s.conns[sc]; !ok {
				continue
			}
			delete(s.conns, sc)
			delete(s.httpReadChannels, sc)
			sc.Close()

		case req := <-s.chHandleHTTPChannel:
			if _, ok := s.conns[req.sc]; !ok {
				continue
			}
			if !req.write {
				req.sc.httpReadTunnelID = req.tunnelID
				s.httpReadChannels[req.sc] = req.res
			} else {
				readChan, readChanRes := s.findHTTPReadChannel(req.sc, req.tunnelID)
				if readChan == nil {
					req.res <- fmt.Errorf("did not found a corresponding HTTP GET request")
				} else {
					delete(s.httpReadChannels, readChan)
					close(readChanRes)
					req.res <- errHTTPUpgraded

					nconn := newServerHTTPTunnel(req.sc.nconn, req.sc.httpReadBuf, readChan.nconn)
					sc := &ServerConn{
						s:      s,
						nconn:  nconn,
						tunnel: TunnelHTTP,
					}
					sc.initialize()
					s.conns[sc] = struct{}{}
				}
			}

		case req := <-s.chHandleRequest:
			ss, ok := s.sessions[req.id]
			if ok {
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

				ss = &ServerSession{
					s:      s,
					author: req.sc,
				}
				ss.initialize()
				s.sessions[ss.secretID] = ss
			}

			ss.handleRequestNoWait(req)

		case ss := <-s.chCloseSession:
			if sss, ok := s.sessions[ss.secretID]; !ok || sss != ss {
				continue
			}
			delete(s.sessions, ss.secretID)
			ss.Close()

		case req := <-s.chGetMulticastIP:
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
}

// StartAndWait starts the server and waits until a fatal error.
func (s *Server) StartAndWait() error {
	err := s.Start()
	if err != nil {
		return err
	}
	defer s.Close()

	return s.Wait()
}

func (s *Server) findHTTPReadChannel(writeChan *ServerConn, tunnelID string) (*ServerConn, chan error) {
	for readChan, readChanRes := range s.httpReadChannels {
		if readChan.remoteAddr.IP.Equal(writeChan.remoteAddr.IP) &&
			readChan.httpReadTunnelID == tunnelID {
			return readChan, readChanRes
		}
	}
	return nil, nil
}

func (s *Server) getMulticastIP() (net.IP, error) {
	res := make(chan net.IP)
	select {
	case s.chGetMulticastIP <- chGetMulticastIPReq{res: res}:
		return <-res, nil

	case <-s.ctx.Done():
		return nil, liberrors.ErrServerTerminated{}
	}
}

func (s *Server) newConn(nconn net.Conn) {
	select {
	case s.chNewConn <- nconn:
	case <-s.ctx.Done():
		nconn.Close()
	}
}

func (s *Server) acceptErr(err error) {
	select {
	case s.chAcceptErr <- err:
	case <-s.ctx.Done():
	}
}

func (s *Server) closeConn(sc *ServerConn) {
	select {
	case s.chCloseConn <- sc:
	case <-s.ctx.Done():
	}
}

func (s *Server) closeSession(ss *ServerSession) {
	select {
	case s.chCloseSession <- ss:
	case <-s.ctx.Done():
	}
}

func (s *Server) handleHTTPChannel(req sessionHandleHTTPChannelReq) error {
	req.res = make(chan error)

	select {
	case s.chHandleHTTPChannel <- req:
	case <-s.ctx.Done():
		return fmt.Errorf("terminated")
	}

	if !req.write {
		t := time.NewTimer(5 * time.Second)
		defer t.Stop()

		select {
		case <-req.res:
		case <-req.sc.ctx.Done():
			return fmt.Errorf("terminated")
		case <-t.C:
			return fmt.Errorf("did not found a corresponding HTTP POST request")
		}
		return errHTTPUpgraded
	}

	return <-req.res
}

func (s *Server) handleRequest(req sessionRequestReq) (*base.Response, *ServerSession, error) {
	select {
	case s.chHandleRequest <- req:
		res := <-req.res
		return res.res, res.ss, res.err

	case <-s.ctx.Done():
		return &base.Response{
			StatusCode: base.StatusBadRequest,
		}, req.sc.session, liberrors.ErrServerTerminated{}
	}
}
