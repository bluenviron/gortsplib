package gortsplib

import (
	"crypto/rand"
	"fmt"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
)

type clientMedia struct {
	c      *Client
	media  *description.Media
	secure bool

	srtpOutCtx             *wrappedSRTPContext
	srtpInCtx              *wrappedSRTPContext
	onPacketRTCP           OnPacketRTCPFunc
	formats                map[uint8]*clientFormat
	tcpChannel             int
	udpRTPListener         *clientUDPListener
	udpRTCPListener        *clientUDPListener
	writePacketRTCPInQueue func([]byte) error
	bytesReceived          *uint64
	bytesSent              *uint64
	rtpPacketsInError      *uint64
	rtcpPacketsReceived    *uint64
	rtcpPacketsSent        *uint64
	rtcpPacketsInError     *uint64
}

func (cm *clientMedia) initialize() error {
	cm.onPacketRTCP = func(rtcp.Packet) {}
	cm.bytesReceived = new(uint64)
	cm.bytesSent = new(uint64)
	cm.rtpPacketsInError = new(uint64)
	cm.rtcpPacketsReceived = new(uint64)
	cm.rtcpPacketsSent = new(uint64)
	cm.rtcpPacketsInError = new(uint64)

	cm.formats = make(map[uint8]*clientFormat)

	for _, forma := range cm.media.Formats {
		f := &clientFormat{
			cm:          cm,
			format:      forma,
			onPacketRTP: func(*rtp.Packet) {},
		}
		err := f.initialize()
		if err != nil {
			return err
		}

		cm.formats[forma.PayloadType()] = f
	}

	if cm.secure {
		var srtpOutKey []byte
		if cm.c.state == clientStatePreRecord {
			srtpOutKey = cm.c.announceData[cm.media].srtpOutKey
		} else {
			srtpOutKey = make([]byte, srtpKeyLength)
			_, err := rand.Read(srtpOutKey)
			if err != nil {
				return err
			}
		}

		ssrcs := make([]uint32, len(cm.formats))
		n := 0
		for _, cf := range cm.formats {
			ssrcs[n] = cf.localSSRC
			n++
		}

		cm.srtpOutCtx = &wrappedSRTPContext{
			key:   srtpOutKey,
			ssrcs: ssrcs,
		}
		err := cm.srtpOutCtx.initialize()
		if err != nil {
			return err
		}
	}

	return nil
}

func (cm *clientMedia) close() {
	if cm.udpRTPListener != nil {
		cm.udpRTPListener.close()
		cm.udpRTCPListener.close()
	}
}

func (cm *clientMedia) createUDPListeners(
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

	// pick two consecutive ports in range 65535-10000
	// RTP port must be even and RTCP port odd
	for {
		v, err := randInRange((65535 - 10000) / 2)
		if err != nil {
			return err
		}

		rtpPort := v*2 + 10000
		rtcpPort := rtpPort + 1

		cm.udpRTPListener = &clientUDPListener{
			c:                 cm.c,
			multicastEnable:   false,
			multicastSourceIP: nil,
			address:           net.JoinHostPort("", strconv.FormatInt(int64(rtpPort), 10)),
		}
		err = cm.udpRTPListener.initialize()
		if err != nil {
			cm.udpRTPListener = nil
			continue
		}

		cm.udpRTCPListener = &clientUDPListener{
			c:                 cm.c,
			multicastEnable:   false,
			multicastSourceIP: nil,
			address:           net.JoinHostPort("", strconv.FormatInt(int64(rtcpPort), 10)),
		}
		err = cm.udpRTCPListener.initialize()
		if err != nil {
			cm.udpRTPListener.close()
			cm.udpRTPListener = nil
			cm.udpRTCPListener = nil
			continue
		}

		return nil
	}
}

func (cm *clientMedia) start() {
	if cm.udpRTPListener != nil {
		cm.writePacketRTCPInQueue = cm.writePacketRTCPInQueueUDP

		if cm.c.state == clientStateRecord || cm.media.IsBackChannel {
			cm.udpRTPListener.readFunc = cm.readPacketRTPUDPRecord
			cm.udpRTCPListener.readFunc = cm.readPacketRTCPUDPRecord
		} else {
			cm.udpRTPListener.readFunc = cm.readPacketRTPUDPPlay
			cm.udpRTCPListener.readFunc = cm.readPacketRTCPUDPPlay
		}
	} else {
		cm.writePacketRTCPInQueue = cm.writePacketRTCPInQueueTCP

		if cm.c.tcpCallbackByChannel == nil {
			cm.c.tcpCallbackByChannel = make(map[int]readFunc)
		}

		if cm.c.state == clientStateRecord || cm.media.IsBackChannel {
			cm.c.tcpCallbackByChannel[cm.tcpChannel] = cm.readPacketRTPTCPRecord
			cm.c.tcpCallbackByChannel[cm.tcpChannel+1] = cm.readPacketRTCPTCPRecord
		} else {
			cm.c.tcpCallbackByChannel[cm.tcpChannel] = cm.readPacketRTPTCPPlay
			cm.c.tcpCallbackByChannel[cm.tcpChannel+1] = cm.readPacketRTCPTCPPlay
		}
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

func (cm *clientMedia) findFormatByRemoteSSRC(ssrc uint32) *clientFormat {
	for _, cf := range cm.formats {
		if v, ok := cf.remoteSSRC(); ok && v == ssrc {
			return cf
		}
	}
	return nil
}

func (cm *clientMedia) decodeRTP(payload []byte) (*rtp.Packet, error) {
	if cm.srtpInCtx != nil {
		var err error
		payload, err = cm.srtpInCtx.decryptRTP(payload, payload, nil)
		if err != nil {
			return nil, err
		}
	}

	var pkt rtp.Packet
	err := pkt.Unmarshal(payload)
	return &pkt, err
}

func (cm *clientMedia) decodeRTCP(payload []byte) ([]rtcp.Packet, error) {
	if cm.srtpInCtx != nil {
		var err error
		payload, err = cm.srtpInCtx.decryptRTCP(payload, payload, nil)
		if err != nil {
			return nil, err
		}
	}

	pkts, err := rtcp.Unmarshal(payload)
	if err != nil {
		return nil, err
	}

	return pkts, nil
}

func (cm *clientMedia) readPacketRTPTCPPlay(payload []byte) bool {
	atomic.AddUint64(cm.bytesReceived, uint64(len(payload)))

	now := cm.c.timeNow()
	atomic.StoreInt64(cm.c.tcpLastFrameTime, now.Unix())

	pkt, err := cm.decodeRTP(payload)
	if err != nil {
		cm.onPacketRTPDecodeError(err)
		return false
	}

	forma, ok := cm.formats[pkt.PayloadType]
	if !ok {
		cm.onPacketRTPDecodeError(liberrors.ErrClientRTPPacketUnknownPayloadType{PayloadType: pkt.PayloadType})
		return false
	}

	forma.readPacketRTPTCP(pkt)

	return true
}

func (cm *clientMedia) readPacketRTCPTCPPlay(payload []byte) bool {
	atomic.AddUint64(cm.bytesReceived, uint64(len(payload)))

	now := cm.c.timeNow()
	atomic.StoreInt64(cm.c.tcpLastFrameTime, now.Unix())

	if len(payload) > udpMaxPayloadSize {
		cm.onPacketRTCPDecodeError(liberrors.ErrClientRTCPPacketTooBig{L: len(payload), Max: udpMaxPayloadSize})
		return false
	}

	packets, err := cm.decodeRTCP(payload)
	if err != nil {
		cm.onPacketRTCPDecodeError(err)
		return false
	}

	atomic.AddUint64(cm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		if sr, ok := pkt.(*rtcp.SenderReport); ok {
			format := cm.findFormatByRemoteSSRC(sr.SSRC)
			if format != nil {
				format.rtcpReceiver.ProcessSenderReport(sr, now)
			}
		}

		cm.onPacketRTCP(pkt)
	}

	return true
}

func (cm *clientMedia) readPacketRTPTCPRecord(_ []byte) bool {
	return false
}

func (cm *clientMedia) readPacketRTCPTCPRecord(payload []byte) bool {
	atomic.AddUint64(cm.bytesReceived, uint64(len(payload)))

	if len(payload) > udpMaxPayloadSize {
		cm.onPacketRTCPDecodeError(liberrors.ErrClientRTCPPacketTooBig{L: len(payload), Max: udpMaxPayloadSize})
		return false
	}

	packets, err := cm.decodeRTCP(payload)
	if err != nil {
		cm.onPacketRTCPDecodeError(err)
		return false
	}

	atomic.AddUint64(cm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		cm.onPacketRTCP(pkt)
	}

	return true
}

func (cm *clientMedia) readPacketRTPUDPPlay(payload []byte) bool {
	atomic.AddUint64(cm.bytesReceived, uint64(len(payload)))

	if len(payload) == (udpMaxPayloadSize + 1) {
		cm.onPacketRTPDecodeError(liberrors.ErrClientRTPPacketTooBigUDP{})
		return false
	}

	pkt, err := cm.decodeRTP(payload)
	if err != nil {
		cm.onPacketRTPDecodeError(err)
		return false
	}

	forma, ok := cm.formats[pkt.PayloadType]
	if !ok {
		cm.onPacketRTPDecodeError(liberrors.ErrClientRTPPacketUnknownPayloadType{PayloadType: pkt.PayloadType})
		return false
	}

	forma.readPacketRTPUDP(pkt)

	return true
}

func (cm *clientMedia) readPacketRTCPUDPPlay(payload []byte) bool {
	atomic.AddUint64(cm.bytesReceived, uint64(len(payload)))

	if len(payload) == (udpMaxPayloadSize + 1) {
		cm.onPacketRTCPDecodeError(liberrors.ErrClientRTCPPacketTooBigUDP{})
		return false
	}

	packets, err := cm.decodeRTCP(payload)
	if err != nil {
		cm.onPacketRTCPDecodeError(err)
		return false
	}

	now := cm.c.timeNow()

	atomic.AddUint64(cm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		if sr, ok := pkt.(*rtcp.SenderReport); ok {
			format := cm.findFormatByRemoteSSRC(sr.SSRC)
			if format != nil {
				format.rtcpReceiver.ProcessSenderReport(sr, now)
			}
		}

		cm.onPacketRTCP(pkt)
	}

	return true
}

func (cm *clientMedia) readPacketRTPUDPRecord(_ []byte) bool {
	return false
}

func (cm *clientMedia) readPacketRTCPUDPRecord(payload []byte) bool {
	atomic.AddUint64(cm.bytesReceived, uint64(len(payload)))

	if len(payload) == (udpMaxPayloadSize + 1) {
		cm.onPacketRTCPDecodeError(liberrors.ErrClientRTCPPacketTooBigUDP{})
		return false
	}

	packets, err := cm.decodeRTCP(payload)
	if err != nil {
		cm.onPacketRTCPDecodeError(err)
		return false
	}

	atomic.AddUint64(cm.rtcpPacketsReceived, uint64(len(packets)))

	for _, pkt := range packets {
		cm.onPacketRTCP(pkt)
	}

	return true
}

func (cm *clientMedia) onPacketRTPDecodeError(err error) {
	atomic.AddUint64(cm.rtpPacketsInError, 1)
	cm.c.OnDecodeError(err)
}

func (cm *clientMedia) onPacketRTCPDecodeError(err error) {
	atomic.AddUint64(cm.rtcpPacketsInError, 1)
	cm.c.OnDecodeError(err)
}

func (cm *clientMedia) writePacketRTCP(pkt rtcp.Packet) error {
	buf, err := pkt.Marshal()
	if err != nil {
		return err
	}

	maxPlainPacketSize := cm.c.MaxPacketSize
	if cm.srtpOutCtx != nil {
		maxPlainPacketSize -= srtcpOverhead
	}

	if len(buf) > maxPlainPacketSize {
		return fmt.Errorf("packet is too big")
	}

	if cm.srtpOutCtx != nil {
		encr := make([]byte, cm.c.MaxPacketSize)
		encr, err = cm.srtpOutCtx.encryptRTCP(encr, buf, nil)
		if err != nil {
			return err
		}
		buf = encr
	}

	ok := cm.c.writer.push(func() error {
		return cm.writePacketRTCPInQueue(buf)
	})
	if !ok {
		return liberrors.ErrClientWriteQueueFull{}
	}

	return nil
}

func (cm *clientMedia) writePacketRTCPInQueueUDP(payload []byte) error {
	err := cm.udpRTCPListener.write(payload)
	if err != nil {
		return err
	}

	atomic.AddUint64(cm.bytesSent, uint64(len(payload)))
	atomic.AddUint64(cm.rtcpPacketsSent, 1)
	return nil
}

func (cm *clientMedia) writePacketRTCPInQueueTCP(payload []byte) error {
	cm.c.tcpFrame.Channel = cm.tcpChannel + 1
	cm.c.tcpFrame.Payload = payload
	cm.c.nconn.SetWriteDeadline(time.Now().Add(cm.c.WriteTimeout))
	err := cm.c.conn.WriteInterleavedFrame(cm.c.tcpFrame, cm.c.tcpBuffer)
	if err != nil {
		return err
	}

	atomic.AddUint64(cm.bytesSent, uint64(len(payload)))
	atomic.AddUint64(cm.rtcpPacketsSent, 1)
	return nil
}
