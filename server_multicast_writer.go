package gortsplib

import (
	"net"

	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
)

type serverMulticastWriter struct {
	s *Server

	rtpl     *serverUDPListener
	rtcpl    *serverUDPListener
	writer   asyncProcessor
	rtpAddr  *net.UDPAddr
	rtcpAddr *net.UDPAddr
}

func (h *serverMulticastWriter) initialize() error {
	ip, err := h.s.getMulticastIP()
	if err != nil {
		return err
	}

	rtpl, rtcpl, err := serverAllocateUDPListenerMulticastPair(
		h.s.ListenPacket,
		h.s.WriteTimeout,
		h.s.MulticastRTPPort,
		h.s.MulticastRTCPPort,
		ip,
	)
	if err != nil {
		return err
	}

	rtpAddr := &net.UDPAddr{
		IP:   rtpl.ip(),
		Port: rtpl.port(),
	}

	rtcpAddr := &net.UDPAddr{
		IP:   rtcpl.ip(),
		Port: rtcpl.port(),
	}

	h.rtpl = rtpl
	h.rtcpl = rtcpl
	h.rtpAddr = rtpAddr
	h.rtcpAddr = rtcpAddr

	h.writer.allocateBuffer(h.s.WriteQueueSize)
	h.writer.start()

	return nil
}

func (h *serverMulticastWriter) close() {
	h.rtpl.close()
	h.rtcpl.close()
	h.writer.stop()
}

func (h *serverMulticastWriter) ip() net.IP {
	return h.rtpl.ip()
}

func (h *serverMulticastWriter) writePacketRTP(payload []byte) error {
	ok := h.writer.push(func() {
		h.rtpl.write(payload, h.rtpAddr) //nolint:errcheck
	})
	if !ok {
		return liberrors.ErrServerWriteQueueFull{}
	}

	return nil
}

func (h *serverMulticastWriter) writePacketRTCP(payload []byte) error {
	ok := h.writer.push(func() {
		h.rtcpl.write(payload, h.rtcpAddr) //nolint:errcheck
	})
	if !ok {
		return liberrors.ErrServerWriteQueueFull{}
	}

	return nil
}
