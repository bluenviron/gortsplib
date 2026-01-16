package gortsplib

import (
	"context"
	"net"
	"time"

	"github.com/bluenviron/gortsplib/v5/internal/asyncprocessor"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
)

type serverMulticastWriter struct {
	udpReadBufferSize int
	listenPacket      func(network string, address string) (net.PacketConn, error)
	writeQueueSize    int
	writeTimeout      time.Duration
	ip                net.IP
	rtpPort           int
	rtcpPort          int

	rtpl     *serverUDPListener
	rtcpl    *serverUDPListener
	writer   *asyncprocessor.Processor
	rtpAddr  *net.UDPAddr
	rtcpAddr *net.UDPAddr
}

func (h *serverMulticastWriter) initialize() error {
	rtpl, rtcpl, err := createUDPListenerMulticastPair(
		h.udpReadBufferSize,
		h.listenPacket,
		h.writeTimeout,
		h.rtpPort,
		h.rtcpPort,
		h.ip,
	)
	if err != nil {
		return err
	}

	rtpAddr := &net.UDPAddr{
		IP:   h.ip,
		Port: h.rtpPort,
	}

	rtcpAddr := &net.UDPAddr{
		IP:   h.ip,
		Port: h.rtcpPort,
	}

	h.rtpl = rtpl
	h.rtcpl = rtcpl
	h.rtpAddr = rtpAddr
	h.rtcpAddr = rtcpAddr

	h.writer = &asyncprocessor.Processor{
		BufferSize: h.writeQueueSize,
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
