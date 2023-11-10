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
	c                      *Client
	media                  *description.Media
	formats                map[uint8]*clientFormat
	tcpChannel             int
	udpRTPListener         *clientUDPListener
	udpRTCPListener        *clientUDPListener
	tcpRTPFrame            *base.InterleavedFrame
	tcpRTCPFrame           *base.InterleavedFrame
	tcpBuffer              []byte
	writePacketRTPInQueue  func([]byte)
	writePacketRTCPInQueue func([]byte)
	onPacketRTCP           OnPacketRTCPFunc
}

func newClientMedia(c *Client) *clientMedia {
	return &clientMedia{
		c:            c,
		onPacketRTCP: func(rtcp.Packet) {},
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
		l1, err := newClientUDPListener(
			cm.c,
			multicastEnable,
			multicastSourceIP,
			rtpAddress,
		)
		if err != nil {
			return err
		}

		l2, err := newClientUDPListener(
			cm.c,
			multicastEnable,
			multicastSourceIP,
			rtcpAddress,
		)
		if err != nil {
			l1.close()
			return err
		}

		cm.udpRTPListener, cm.udpRTCPListener = l1, l2
		return nil
	}

	var err error
	cm.udpRTPListener, cm.udpRTCPListener, err = newClientUDPListenerPair(cm.c)
	return err
}

func (cm *clientMedia) setMedia(medi *description.Media) {
	cm.media = medi

	cm.formats = make(map[uint8]*clientFormat)
	for _, forma := range medi.Formats {
		cm.formats[forma.PayloadType()] = newClientFormat(cm, forma)
	}
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
		tssrc, ok := format.rtcpReceiver.SenderSSRC()
		if ok && tssrc == ssrc {
			return format
		}
	}
	return nil
}

func (cm *clientMedia) writePacketRTPInQueueUDP(payload []byte) {
	atomic.AddUint64(cm.c.BytesSent, uint64(len(payload)))
	cm.udpRTPListener.write(payload) //nolint:errcheck
}

func (cm *clientMedia) writePacketRTCPInQueueUDP(payload []byte) {
	atomic.AddUint64(cm.c.BytesSent, uint64(len(payload)))
	cm.udpRTCPListener.write(payload) //nolint:errcheck
}

func (cm *clientMedia) writePacketRTPInQueueTCP(payload []byte) {
	atomic.AddUint64(cm.c.BytesSent, uint64(len(payload)))
	cm.tcpRTPFrame.Payload = payload
	cm.c.nconn.SetWriteDeadline(time.Now().Add(cm.c.WriteTimeout))
	cm.c.conn.WriteInterleavedFrame(cm.tcpRTPFrame, cm.tcpBuffer) //nolint:errcheck
}

func (cm *clientMedia) writePacketRTCPInQueueTCP(payload []byte) {
	atomic.AddUint64(cm.c.BytesSent, uint64(len(payload)))
	cm.tcpRTCPFrame.Payload = payload
	cm.c.nconn.SetWriteDeadline(time.Now().Add(cm.c.WriteTimeout))
	cm.c.conn.WriteInterleavedFrame(cm.tcpRTCPFrame, cm.tcpBuffer) //nolint:errcheck
}

func (cm *clientMedia) writePacketRTCP(byts []byte) error {
	ok := cm.c.writer.push(func() {
		cm.writePacketRTCPInQueue(byts)
	})
	if !ok {
		return liberrors.ErrClientWriteQueueFull{}
	}

	return nil
}

func (cm *clientMedia) readRTPTCPPlay(payload []byte) {
	now := cm.c.timeNow()
	atomic.StoreInt64(cm.c.tcpLastFrameTime, now.Unix())

	pkt := &rtp.Packet{}
	err := pkt.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return
	}

	forma, ok := cm.formats[pkt.PayloadType]
	if !ok {
		cm.c.OnDecodeError(liberrors.ErrClientRTPPacketUnknownPayloadType{PayloadType: pkt.PayloadType})
		return
	}

	forma.readRTPTCP(pkt)
}

func (cm *clientMedia) readRTCPTCPPlay(payload []byte) {
	now := cm.c.timeNow()
	atomic.StoreInt64(cm.c.tcpLastFrameTime, now.Unix())

	if len(payload) > udpMaxPayloadSize {
		cm.c.OnDecodeError(liberrors.ErrClientRTCPPacketTooBig{L: len(payload), Max: udpMaxPayloadSize})
		return
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return
	}

	for _, pkt := range packets {
		if sr, ok := pkt.(*rtcp.SenderReport); ok {
			format := cm.findFormatWithSSRC(sr.SSRC)
			if format != nil {
				format.rtcpReceiver.ProcessSenderReport(sr, now)
			}
		}

		cm.onPacketRTCP(pkt)
	}
}

func (cm *clientMedia) readRTPTCPRecord(_ []byte) {
}

func (cm *clientMedia) readRTCPTCPRecord(payload []byte) {
	if len(payload) > udpMaxPayloadSize {
		cm.c.OnDecodeError(liberrors.ErrClientRTCPPacketTooBig{L: len(payload), Max: udpMaxPayloadSize})
		return
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return
	}

	for _, pkt := range packets {
		cm.onPacketRTCP(pkt)
	}
}

func (cm *clientMedia) readRTPUDPPlay(payload []byte) {
	plen := len(payload)

	atomic.AddUint64(cm.c.BytesReceived, uint64(plen))

	if plen == (udpMaxPayloadSize + 1) {
		cm.c.OnDecodeError(liberrors.ErrClientRTPPacketTooBigUDP{})
		return
	}

	pkt := &rtp.Packet{}
	err := pkt.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return
	}

	forma, ok := cm.formats[pkt.PayloadType]
	if !ok {
		cm.c.OnDecodeError(liberrors.ErrClientRTPPacketUnknownPayloadType{PayloadType: pkt.PayloadType})
		return
	}

	forma.readRTPUDP(pkt)
}

func (cm *clientMedia) readRTCPUDPPlay(payload []byte) {
	now := cm.c.timeNow()
	plen := len(payload)

	atomic.AddUint64(cm.c.BytesReceived, uint64(plen))

	if plen == (udpMaxPayloadSize + 1) {
		cm.c.OnDecodeError(liberrors.ErrClientRTCPPacketTooBigUDP{})
		return
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return
	}

	for _, pkt := range packets {
		if sr, ok := pkt.(*rtcp.SenderReport); ok {
			format := cm.findFormatWithSSRC(sr.SSRC)
			if format != nil {
				format.rtcpReceiver.ProcessSenderReport(sr, now)
			}
		}

		cm.onPacketRTCP(pkt)
	}
}

func (cm *clientMedia) readRTPUDPRecord(_ []byte) {
}

func (cm *clientMedia) readRTCPUDPRecord(payload []byte) {
	plen := len(payload)

	atomic.AddUint64(cm.c.BytesReceived, uint64(plen))

	if plen == (udpMaxPayloadSize + 1) {
		cm.c.OnDecodeError(liberrors.ErrClientRTCPPacketTooBigUDP{})
		return
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return
	}

	for _, pkt := range packets {
		cm.onPacketRTCP(pkt)
	}
}
