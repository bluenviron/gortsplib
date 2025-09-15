package gortsplib

import (
	"context"
	"net"

	"github.com/bluenviron/gortsplib/v5/internal/asyncprocessor"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
)

type serverMulticastWriter struct {
	s *Server

	rtpl     *serverUDPListener
	rtcpl    *serverUDPListener
	writer   *asyncprocessor.Processor
	rtpAddr  *net.UDPAddr
	rtcpAddr *net.UDPAddr
}

func (h *serverMulticastWriter) initialize() error {
	ip, err := h.s.getMulticastIP()
	if err != nil {
		return err
	}

	rtpl, rtcpl, err := createUDPListenerMulticastPair(
		h.s.UDPReadBufferSize,
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

	h.writer = &asyncprocessor.Processor{
		BufferSize: h.s.WriteQueueSize,
		OnError:    func(_ context.Context, _ error) {},
	}
	h.writer.Initialize()
	h.writer.Start()

	return nil
}

func (h *serverMulticastWriter) close() {
	h.rtpl.close()
	h.rtcpl.close()
	h.writer.Close()
}

func (h *serverMulticastWriter) ip() net.IP {
	return h.rtpl.ip()
}

func (h *serverMulticastWriter) writePacketRTP(byts []byte) error {
	ok := h.writer.Push(func() error {
		return h.rtpl.write(byts, h.rtpAddr)
	})
	if !ok {
		return liberrors.ErrServerWriteQueueFull{}
	}

	return nil
}

func (h *serverMulticastWriter) writePacketRTCP(byts []byte) error {
	ok := h.writer.Push(func() error {
		return h.rtcpl.write(byts, h.rtcpAddr)
	})
	if !ok {
		return liberrors.ErrServerWriteQueueFull{}
	}

	return nil
}
