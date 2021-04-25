package gortsplib

import (
	"crypto/tls"
	"fmt"
	"net"
	"strconv"
	"time"
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

// Server is a RTSP server.
type Server struct {
	// a TLS configuration to accept TLS (RTSPS) connections.
	TLSConfig *tls.Config

	// a port to send and receive UDP/RTP packets.
	// If UDPRTPAddress and UDPRTCPAddress are != "", the server can accept and send UDP streams.
	UDPRTPAddress string

	// a port to send and receive UDP/RTCP packets.
	// If UDPRTPAddress and UDPRTCPAddress are != "", the server can accept and send UDP streams.
	UDPRTCPAddress string

	// timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration

	// timeout of write operations.
	// It defaults to 10 seconds
	WriteTimeout time.Duration

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

	// function used to initialize the TCP listener.
	// It defaults to net.Listen
	Listen func(network string, address string) (net.Listener, error)

	receiverReportPeriod time.Duration

	tcpListener     net.Listener
	udpRTPListener  *serverUDPListener
	udpRTCPListener *serverUDPListener
}

// Serve starts listening on the given address.
func (s *Server) Serve(address string) error {
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

	if s.Listen == nil {
		s.Listen = net.Listen
	}

	if s.receiverReportPeriod == 0 {
		s.receiverReportPeriod = 10 * time.Second
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
			return err
		}
	}

	var err error
	s.tcpListener, err = s.Listen("tcp", address)
	if err != nil {
		return err
	}

	return nil
}

// Accept accepts a connection.
func (s *Server) Accept() (*ServerConn, error) {
	nconn, err := s.tcpListener.Accept()
	if err != nil {
		return nil, err
	}

	return newServerConn(s, nconn), nil
}

// Close closes all the server resources.
func (s *Server) Close() error {
	s.tcpListener.Close()

	if s.udpRTPListener != nil {
		s.udpRTPListener.close()
	}

	if s.udpRTCPListener != nil {
		s.udpRTCPListener.close()
	}

	return nil
}
