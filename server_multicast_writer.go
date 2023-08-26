package gortsplib

import (
	"net"
)

type serverMulticastWriter struct {
	rtpl     *serverUDPListener
	rtcpl    *serverUDPListener
	writer   asyncProcessor
	rtpAddr  *net.UDPAddr
	rtcpAddr *net.UDPAddr
}

func newServerMulticastWriter(s *Server) (*serverMulticastWriter, error) {
	ip, err := s.getMulticastIP()
	if err != nil {
		return nil, err
	}

	rtpl, rtcpl, err := newServerUDPListenerMulticastPair(
		s.ListenPacket,
		s.WriteTimeout,
		s.MulticastRTPPort,
		s.MulticastRTCPPort,
		ip,
	)
	if err != nil {
		return nil, err
	}

	rtpAddr := &net.UDPAddr{
		IP:   rtpl.ip(),
		Port: rtpl.port(),
	}

	rtcpAddr := &net.UDPAddr{
		IP:   rtcpl.ip(),
		Port: rtcpl.port(),
	}

	h := &serverMulticastWriter{
		rtpl:     rtpl,
		rtcpl:    rtcpl,
		rtpAddr:  rtpAddr,
		rtcpAddr: rtcpAddr,
	}

	h.writer.allocateBuffer(s.WriteBufferCount)
	h.writer.start()

	return h, nil
}

func (h *serverMulticastWriter) close() {
	h.rtpl.close()
	h.rtcpl.close()
	h.writer.stop()
}

func (h *serverMulticastWriter) ip() net.IP {
	return h.rtpl.ip()
}

func (h *serverMulticastWriter) writePacketRTP(payload []byte) {
	h.writer.queue(func() {
		h.rtpl.write(payload, h.rtpAddr) //nolint:errcheck
	})
}

func (h *serverMulticastWriter) writePacketRTCP(payload []byte) {
	h.writer.queue(func() {
		h.rtcpl.write(payload, h.rtcpAddr) //nolint:errcheck
	})
}
