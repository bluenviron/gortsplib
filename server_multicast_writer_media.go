package gortsplib

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/pion/rtcp"

	"github.com/bluenviron/gortsplib/v5/internal/asyncprocessor"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
)

type serverMulticastWriterMedia struct {
	media             *description.Media
	maxPacketSize     int
	udpReadBufferSize int
	listenPacket      func(network string, address string) (net.PacketConn, error)
	writeQueueSize    int
	writeTimeout      time.Duration
	ip                net.IP
	rtpPort           int
	rtcpPort          int
	srtpOutCtx        *wrappedSRTPContext

	rtpl     *serverUDPListener
	rtcpl    *serverUDPListener
	writer   *asyncprocessor.Processor
	rtpAddr  *net.UDPAddr
	rtcpAddr *net.UDPAddr
	formats  map[uint8]*serverMulticastWriterFormat
}

func (smm *serverMulticastWriterMedia) initialize() error {
	rtpl, rtcpl, err := createUDPListenerMulticastPair(
		smm.udpReadBufferSize,
		smm.listenPacket,
		smm.writeTimeout,
		smm.rtpPort,
		smm.rtcpPort,
		smm.ip,
	)
	if err != nil {
		return err
	}

	rtpAddr := &net.UDPAddr{
		IP:   smm.ip,
		Port: smm.rtpPort,
	}

	rtcpAddr := &net.UDPAddr{
		IP:   smm.ip,
		Port: smm.rtcpPort,
	}

	smm.rtpl = rtpl
	smm.rtcpl = rtcpl
	smm.rtpAddr = rtpAddr
	smm.rtcpAddr = rtcpAddr

	smm.writer = &asyncprocessor.Processor{
		BufferSize: smm.writeQueueSize,
		OnError: func(_ context.Context, _ error) {
			panic("should not happen")
		},
	}
	smm.writer.Initialize()
	smm.writer.Start()

	smm.formats = make(map[uint8]*serverMulticastWriterFormat)

	return nil
}

func (smm *serverMulticastWriterMedia) close() {
	for _, smf := range smm.formats {
		smf.close()
	}
	smm.rtpl.close()
	smm.rtcpl.close()
	smm.writer.Close()
}

func (smm *serverMulticastWriterMedia) writePacketRTCP(pkt rtcp.Packet) error {
	plain, err := pkt.Marshal()
	if err != nil {
		return err
	}

	maxPlainPacketSize := smm.maxPacketSize
	if smm.srtpOutCtx != nil {
		maxPlainPacketSize -= srtcpOverhead
	}

	if len(plain) > maxPlainPacketSize {
		return fmt.Errorf("packet is too big")
	}

	var encr []byte
	if smm.srtpOutCtx != nil {
		encr = make([]byte, smm.maxPacketSize)
		encr, err = smm.srtpOutCtx.encryptRTCP(encr, plain, nil)
		if err != nil {
			return err
		}
	}

	if smm.srtpOutCtx != nil {
		return smm.writePacketRTCPEncoded(encr)
	}
	return smm.writePacketRTCPEncoded(plain)
}

func (smm *serverMulticastWriterMedia) writePacketRTCPEncoded(payload []byte) error {
	ok := smm.writer.Push(func() error {
		smm.rtcpl.write(payload, smm.rtcpAddr) //nolint:errcheck

		// do not report write errors for two reasons:
		// - errors make the async processor stop silently, and there's no error reporting mechanism
		// - we're writing to several interfaces at once, and the error might be located on a single one
		return nil
	})
	if !ok {
		return liberrors.ErrServerWriteQueueFull{}
	}

	return nil
}
