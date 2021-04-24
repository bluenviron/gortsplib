package gortsplib

import (
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
	conf            ServerConf
	tcpListener     net.Listener
	udpRTPListener  *serverUDPListener
	udpRTCPListener *serverUDPListener
}

func newServer(conf ServerConf, address string) (*Server, error) {
	if conf.ReadTimeout == 0 {
		conf.ReadTimeout = 10 * time.Second
	}
	if conf.WriteTimeout == 0 {
		conf.WriteTimeout = 10 * time.Second
	}
	if conf.ReadBufferCount == 0 {
		conf.ReadBufferCount = 512
	}
	if conf.ReadBufferSize == 0 {
		conf.ReadBufferSize = 2048
	}

	if conf.Listen == nil {
		conf.Listen = net.Listen
	}

	if conf.receiverReportPeriod == 0 {
		conf.receiverReportPeriod = 10 * time.Second
	}

	if conf.TLSConfig != nil && conf.UDPRTPAddress != "" {
		return nil, fmt.Errorf("TLS can't be used together with UDP")
	}

	if (conf.UDPRTPAddress != "" && conf.UDPRTCPAddress == "") ||
		(conf.UDPRTPAddress == "" && conf.UDPRTCPAddress != "") {
		return nil, fmt.Errorf("UDPRTPAddress and UDPRTCPAddress must be used together")
	}

	s := &Server{
		conf: conf,
	}

	if conf.UDPRTPAddress != "" {
		rtpPort, err := extractPort(conf.UDPRTPAddress)
		if err != nil {
			return nil, err
		}

		rtcpPort, err := extractPort(conf.UDPRTCPAddress)
		if err != nil {
			return nil, err
		}

		if (rtpPort % 2) != 0 {
			return nil, fmt.Errorf("RTP port must be even")
		}

		if rtcpPort != (rtpPort + 1) {
			return nil, fmt.Errorf("RTCP and RTP ports must be consecutive")
		}

		s.udpRTPListener, err = newServerUDPListener(conf, conf.UDPRTPAddress, StreamTypeRTP)
		if err != nil {
			return nil, err
		}

		s.udpRTCPListener, err = newServerUDPListener(conf, conf.UDPRTCPAddress, StreamTypeRTCP)
		if err != nil {
			return nil, err
		}
	}

	var err error
	s.tcpListener, err = conf.Listen("tcp", address)
	if err != nil {
		return nil, err
	}

	return s, nil
}

// Close closes the server.
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

// Accept accepts a connection.
func (s *Server) Accept() (*ServerConn, error) {
	nconn, err := s.tcpListener.Accept()
	if err != nil {
		return nil, err
	}

	return newServerConn(s.conf, s.udpRTPListener, s.udpRTCPListener, nconn), nil
}
