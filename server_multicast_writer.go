package gortsplib

import (
	"fmt"
	"net"

	"github.com/aler9/gortsplib/v2/pkg/ringbuffer"
)

type typeAndPayload struct {
	isRTP   bool
	payload []byte
}

type serverMulticastWriter struct {
	rtpl        *serverUDPListener
	rtcpl       *serverUDPListener
	writeBuffer *ringbuffer.RingBuffer

	writerDone chan struct{}
}

func newServerMulticastWriter(s *Server) (*serverMulticastWriter, error) {
	res := make(chan net.IP)
	select {
	case s.streamMulticastIP <- streamMulticastIPReq{res: res}:
	case <-s.ctx.Done():
		return nil, fmt.Errorf("terminated")
	}
	ip := <-res

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

	wb, _ := ringbuffer.New(uint64(s.WriteBufferCount))

	h := &serverMulticastWriter{
		rtpl:        rtpl,
		rtcpl:       rtcpl,
		writeBuffer: wb,
		writerDone:  make(chan struct{}),
	}

	go h.runWriter()

	return h, nil
}

func (h *serverMulticastWriter) close() {
	h.rtpl.close()
	h.rtcpl.close()
	h.writeBuffer.Close()
	<-h.writerDone
}

func (h *serverMulticastWriter) ip() net.IP {
	return h.rtpl.ip()
}

func (h *serverMulticastWriter) runWriter() {
	defer close(h.writerDone)

	rtpAddr := &net.UDPAddr{
		IP:   h.rtpl.ip(),
		Port: h.rtpl.port(),
	}

	rtcpAddr := &net.UDPAddr{
		IP:   h.rtcpl.ip(),
		Port: h.rtcpl.port(),
	}

	for {
		tmp, ok := h.writeBuffer.Pull()
		if !ok {
			return
		}
		data := tmp.(typeAndPayload)

		if data.isRTP {
			h.rtpl.write(data.payload, rtpAddr)
		} else {
			h.rtcpl.write(data.payload, rtcpAddr)
		}
	}
}

func (h *serverMulticastWriter) writePacketRTP(payload []byte) {
	h.writeBuffer.Push(typeAndPayload{
		isRTP:   true,
		payload: payload,
	})
}

func (h *serverMulticastWriter) writePacketRTCP(payload []byte) {
	h.writeBuffer.Push(typeAndPayload{
		isRTP:   false,
		payload: payload,
	})
}
