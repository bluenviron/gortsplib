package gortsplib

import (
	"net"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
)

type clientMedia struct {
	c     *Client
	media *description.Media

	onPacketRTCP           OnPacketRTCPFunc
	formats                map[uint8]*clientFormat
	tcpChannel             int
	udpRTPListener         *clientUDPListener
	udpRTCPListener        *clientUDPListener
	tcpRTPFrame            *base.InterleavedFrame
	tcpRTCPFrame           *base.InterleavedFrame
	tcpBuffer              []byte
	writePacketRTPInQueue  func([]byte) error
	writePacketRTCPInQueue func([]byte) error
	rtpPacketsReceived     *uint64
	rtpPacketsSent         *uint64
	rtpPacketsLost         *uint64
	rtpPacketsInError      *uint64
	rtcpPacketsReceived    *uint64
	rtcpPacketsSent        *uint64
	rtcpPacketsInError     *uint64
}

func (cm *clientMedia) initialize() {
	cm.onPacketRTCP = func(rtcp.Packet) {}
	cm.rtpPacketsReceived = new(uint64)
	cm.rtpPacketsSent = new(uint64)
	cm.rtpPacketsLost = new(uint64)
	cm.rtpPacketsInError = new(uint64)
	cm.rtcpPacketsReceived = new(uint64)
	cm.rtcpPacketsSent = new(uint64)
	cm.rtcpPacketsInError = new(uint64)

	cm.formats = make(map[uint8]*clientFormat)
	for _, forma := range cm.media.Formats {
		cm.formats[forma.PayloadType()] = &clientFormat{
			cm:          cm,
			format:      forma,
			onPacketRTP: func(*rtp.Packet) {},
		}
	}
}

func (cm *clientMedia) close() {
	if cm.udpRTPListener != nil {
		cm.udpRTPListener.close()
		cm.udpRTCPListener.close()
	}
}

func (cm *clientMedia) allocateUDPListeners(
	multicastEnable bool,
	multicastSourceIP net.IP,
	rtpAddress string,
	rtcpAddress string,
) error {
	if rtpAddress != ":0" {
		l1 := &clientUDPListener{
			c:                 cm.c,
			multicastEnable:   multicastEnable,
			multicastSourceIP: multicastSourceIP,
			address:           rtpAddress,
		}
		err := l1.initialize()
		if err != nil {
			return err
		}

		l2 := &clientUDPListener{
			c:                 cm.c,
			multicastEnable:   multicastEnable,
			multicastSourceIP: multicastSourceIP,
			address:           rtcpAddress,
		}
		err = l2.initialize()
		if err != nil {
			l1.close()
			return err
		}

		cm.udpRTPListener, cm.udpRTCPListener = l1, l2
		return nil
	}

	var err error
	cm.udpRTPListener, cm.udpRTCPListener, err = allocateUDPListenerPair(cm.c)
	return err
}

func (cm *clientMedia) start() {
	if cm.udpRTPListener != nil {
		cm.writePacketRTPInQueue = cm.writePacketRTPInQueueUDP
		cm.writePacketRTCPInQueue = cm.writePacketRTCPInQueueUDP

		if cm.c.state == clientStateRecord || cm.media.IsBackChannel {
			cm.udpRTPListener.readFunc = cm.readRTPUDPRecord
			cm.udpRTCPListener.readFunc = cm.readRTCPUDPRecord
		} else {
			cm.udpRTPListener.readFunc = cm.readRTPUDPPlay
			cm.udpRTCPListener.readFunc = cm.readRTCPUDPPlay
		}
	} else {
		cm.writePacketRTPInQueue = cm.writePacketRTPInQueueTCP
		cm.writePacketRTCPInQueue = cm.writePacketRTCPInQueueTCP

		if cm.c.tcpCallbackByChannel == nil {
			cm.c.tcpCallbackByChannel = make(map[int]readFunc)
		}

		if cm.c.state == clientStateRecord || cm.media.IsBackChannel {
			cm.c.tcpCallbackByChannel[cm.tcpChannel] = cm.readRTPTCPRecord
			cm.c.tcpCallbackByChannel[cm.tcpChannel+1] = cm.readRTCPTCPRecord
		} else {
			cm.c.tcpCallbackByChannel[cm.tcpChannel] = cm.readRTPTCPPlay
			cm.c.tcpCallbackByChannel[cm.tcpChannel+1] = cm.readRTCPTCPPlay
		}

		cm.tcpRTPFrame = &base.InterleavedFrame{Channel: cm.tcpChannel}
		cm.tcpRTCPFrame = &base.InterleavedFrame{Channel: cm.tcpChannel + 1}
		cm.tcpBuffer = make([]byte, cm.c.MaxPacketSize+4)
	}

	for _, ct := range cm.formats {
		ct.start()
	}

	if cm.udpRTPListener != nil {
		cm.udpRTPListener.start()
		cm.udpRTCPListener.start()
	}
}

func (cm *clientMedia) stop() {
	if cm.udpRTPListener != nil {
		cm.udpRTPListener.stop()
		cm.udpRTCPListener.stop()
	}

	for _, ct := range cm.formats {
		ct.stop()
	}
}

func (cm *clientMedia) findFormatWithSSRC(ssrc uint32) *clientFormat {
	for _, format := range cm.formats {
		stats := format.rtcpReceiver.Stats()
		if stats != nil && stats.RemoteSSRC == ssrc {
			return format
		}
	}
	return nil
}

func (cm *clientMedia) writePacketRTPInQueueUDP(payload []byte) error {
	err := cm.udpRTPListener.write(payload)
	if err != nil {
		return err
	}

	atomic.AddUint64(cm.c.bytesSent, uint64(len(payload)))
	atomic.AddUint64(cm.rtpPacketsSent, 1)
	return nil
}

func (cm *clientMedia) writePacketRTCPInQueueUDP(payload []byte) error {
	err := cm.udpRTCPListener.write(payload)
	if err != nil {
		return err
	}

	atomic.AddUint64(cm.c.bytesSent, uint64(len(payload)))
	atomic.AddUint64(cm.rtcpPacketsSent, 1)
	return nil
}

func (cm *clientMedia) writePacketRTPInQueueTCP(payload []byte) error {
	cm.tcpRTPFrame.Payload = payload
	cm.c.nconn.SetWriteDeadline(time.Now().Add(cm.c.WriteTimeout))
	err := cm.c.conn.WriteInterleavedFrame(cm.tcpRTPFrame, cm.tcpBuffer)
	if err != nil {
		return err
	}

	atomic.AddUint64(cm.rtpPacketsSent, 1)
	return nil
}

func (cm *clientMedia) writePacketRTCPInQueueTCP(payload []byte) error {
	cm.tcpRTCPFrame.Payload = payload
	cm.c.nconn.SetWriteDeadline(time.Now().Add(cm.c.WriteTimeout))
	err := cm.c.conn.WriteInterleavedFrame(cm.tcpRTCPFrame, cm.tcpBuffer)
	if err != nil {
		return err
	}

	atomic.AddUint64(cm.rtcpPacketsSent, 1)
	return nil
}

func (cm *clientMedia) writePacketRTCP(byts []byte) error {
	ok := cm.c.writer.push(func() error {
		return cm.writePacketRTCPInQueue(byts)
	})
	if !ok {
		return liberrors.ErrClientWriteQueueFull{}
	}

	return nil
}

func (cm *clientMedia) readRTPTCPPlay(payload []byte) bool {
	now := cm.c.timeNow()
	atomic.StoreInt64(cm.c.tcpLastFrameTime, now.Unix())

	pkt := &rtp.Packet{}
	err := pkt.Unmarshal(payload)
	if err != nil {
		cm.onRTPDecodeError(err)
		return false
	}

	forma, ok := cm.formats[pkt.PayloadType]
	if !ok {
		cm.onRTPDecodeError(liberrors.ErrClientRTPPacketUnknownPayloadType{PayloadType: pkt.PayloadType})
		return false
	}

	forma.readRTPTCP(pkt)

	return true
}

func (cm *clientMedia) readRTCPTCPPlay(payload []byte) bool {
	now := cm.c.timeNow()
	atomic.StoreInt64(cm.c.tcpLastFrameTime, now.Unix())

	if len(payload) > udpMaxPayloadSize {
		cm.onRTCPDecodeError(liberrors.ErrClientRTCPPacketTooBig{L: len(payload), Max: udpMaxPayloadSize})
		return false
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.onRTCPDecodeError(err)
		return false
	}

	atomic.AddUint64(cm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		if sr, ok := pkt.(*rtcp.SenderReport); ok {
			format := cm.findFormatWithSSRC(sr.SSRC)
			if format != nil {
				format.rtcpReceiver.ProcessSenderReport(sr, now)
			}
		}

		cm.onPacketRTCP(pkt)
	}

	return true
}

func (cm *clientMedia) readRTPTCPRecord(_ []byte) bool {
	return false
}

func (cm *clientMedia) readRTCPTCPRecord(payload []byte) bool {
	if len(payload) > udpMaxPayloadSize {
		cm.onRTCPDecodeError(liberrors.ErrClientRTCPPacketTooBig{L: len(payload), Max: udpMaxPayloadSize})
		return false
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.onRTCPDecodeError(err)
		return false
	}

	atomic.AddUint64(cm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		cm.onPacketRTCP(pkt)
	}

	return true
}

func (cm *clientMedia) readRTPUDPPlay(payload []byte) bool {
	plen := len(payload)

	if plen == (udpMaxPayloadSize + 1) {
		cm.onRTPDecodeError(liberrors.ErrClientRTPPacketTooBigUDP{})
		return false
	}

	pkt := &rtp.Packet{}
	err := pkt.Unmarshal(payload)
	if err != nil {
		cm.onRTPDecodeError(err)
		return false
	}

	forma, ok := cm.formats[pkt.PayloadType]
	if !ok {
		cm.onRTPDecodeError(liberrors.ErrClientRTPPacketUnknownPayloadType{PayloadType: pkt.PayloadType})
		return false
	}

	forma.readRTPUDP(pkt)

	return true
}

func (cm *clientMedia) readRTCPUDPPlay(payload []byte) bool {
	now := cm.c.timeNow()
	plen := len(payload)

	if plen == (udpMaxPayloadSize + 1) {
		cm.onRTCPDecodeError(liberrors.ErrClientRTCPPacketTooBigUDP{})
		return false
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.onRTCPDecodeError(err)
		return false
	}

	atomic.AddUint64(cm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		if sr, ok := pkt.(*rtcp.SenderReport); ok {
			format := cm.findFormatWithSSRC(sr.SSRC)
			if format != nil {
				format.rtcpReceiver.ProcessSenderReport(sr, now)
			}
		}

		cm.onPacketRTCP(pkt)
	}

	return true
}

func (cm *clientMedia) readRTPUDPRecord(_ []byte) bool {
	return false
}

func (cm *clientMedia) readRTCPUDPRecord(payload []byte) bool {
	plen := len(payload)

	if plen == (udpMaxPayloadSize + 1) {
		cm.onRTCPDecodeError(liberrors.ErrClientRTCPPacketTooBigUDP{})
		return false
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.onRTCPDecodeError(err)
		return false
	}

	atomic.AddUint64(cm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		cm.onPacketRTCP(pkt)
	}

	return true
}

func (cm *clientMedia) onRTPDecodeError(err error) {
	atomic.AddUint64(cm.rtpPacketsInError, 1)
	cm.c.OnDecodeError(err)
}

func (cm *clientMedia) onRTCPDecodeError(err error) {
	atomic.AddUint64(cm.rtcpPacketsInError, 1)
	cm.c.OnDecodeError(err)
}
