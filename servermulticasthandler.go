package gortsplib

import (
	"net"

	"github.com/aler9/gortsplib/v2/pkg/ringbuffer"
)

type typeAndPayload struct {
	isRTP   bool
	payload []byte
}

type serverMulticastHandler struct {
	rtpl        *serverUDPListener
	rtcpl       *serverUDPListener
	writeBuffer *ringbuffer.RingBuffer

	writerDone chan struct{}
}

func newServerMulticastHandler(s *Server) (*serverMulticastHandler, error) {
	rtpl, rtcpl, err := newServerUDPListenerMulticastPair(s)
	if err != nil {
		return nil, err
	}

	wb, _ := ringbuffer.New(uint64(s.WriteBufferCount))

	h := &serverMulticastHandler{
		rtpl:        rtpl,
		rtcpl:       rtcpl,
		writeBuffer: wb,
		writerDone:  make(chan struct{}),
	}

	go h.runWriter()

	return h, nil
}

func (h *serverMulticastHandler) close() {
	h.rtpl.close()
	h.rtcpl.close()
	h.writeBuffer.Close()
	<-h.writerDone
}

func (h *serverMulticastHandler) ip() net.IP {
	return h.rtpl.ip()
}

func (h *serverMulticastHandler) runWriter() {
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

func (h *serverMulticastHandler) writePacketRTP(payload []byte) {
	h.writeBuffer.Push(typeAndPayload{
		isRTP:   true,
		payload: payload,
	})
}

func (h *serverMulticastHandler) writePacketRTCP(payload []byte) {
	h.writeBuffer.Push(typeAndPayload{
		isRTP:   false,
		payload: payload,
	})
}
