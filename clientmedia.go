package gortsplib

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/pion/rtcp"

	"github.com/aler9/gortsplib/v2/pkg/base"
	"github.com/aler9/gortsplib/v2/pkg/media"
)

type clientMedia struct {
	c                      *Client
	media                  *media.Media
	tracks                 map[uint8]*clientTrack
	tcpChannel             int
	udpRTPListener         *clientUDPListener
	udpRTCPListener        *clientUDPListener
	tcpRTPFrame            *base.InterleavedFrame
	tcpRTCPFrame           *base.InterleavedFrame
	tcpBuffer              []byte
	writePacketRTPInQueue  func([]byte)
	writePacketRTCPInQueue func([]byte)
	readRTP                func([]byte) error
	readRTCP               func([]byte) error
	onPacketRTCP           func(rtcp.Packet)
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

func (cm *clientMedia) allocateUDPListeners(multicast bool, rtpAddress string, rtcpAddress string) error {
	if rtpAddress != ":0" {
		l1, err := newClientUDPListener(
			cm.c, multicast, rtpAddress,
			cm, true)
		if err != nil {
			return err
		}

		l2, err := newClientUDPListener(
			cm.c, multicast, rtcpAddress,
			cm, false)
		if err != nil {
			l1.close()
			return err
		}

		cm.udpRTPListener, cm.udpRTCPListener = l1, l2
		return nil
	}

	cm.udpRTPListener, cm.udpRTCPListener = newClientUDPListenerPair(cm.c, cm)
	return nil
}

func (cm *clientMedia) setMedia(medi *media.Media) {
	cm.media = medi

	cm.tracks = make(map[uint8]*clientTrack)
	for _, trak := range medi.Tracks {
		cm.tracks[trak.PayloadType()] = newClientTrack(cm, trak)
	}
}

func (cm *clientMedia) start() {
	if cm.udpRTPListener != nil {
		cm.writePacketRTPInQueue = cm.writePacketRTPInQueueUDP
		cm.writePacketRTCPInQueue = cm.writePacketRTCPInQueueUDP

		if cm.c.state == clientStatePlay {
			cm.readRTP = cm.readRTPUDPPlay
			cm.readRTCP = cm.readRTCPUDPPlay
		} else {
			cm.readRTP = cm.readRTPUDPRecord
			cm.readRTCP = cm.readRTCPUDPRecord
		}
	} else {
		cm.writePacketRTPInQueue = cm.writePacketRTPInQueueTCP
		cm.writePacketRTCPInQueue = cm.writePacketRTCPInQueueTCP

		if cm.c.state == clientStatePlay {
			cm.readRTP = cm.readRTPTCPPlay
			cm.readRTCP = cm.readRTCPTCPPlay
		} else {
			cm.readRTP = cm.readRTPTCPRecord
			cm.readRTCP = cm.readRTCPTCPRecord
		}

		cm.tcpRTPFrame = &base.InterleavedFrame{Channel: cm.tcpChannel}
		cm.tcpRTCPFrame = &base.InterleavedFrame{Channel: cm.tcpChannel + 1}
		cm.tcpBuffer = make([]byte, maxPacketSize+4)
	}

	for _, ct := range cm.tracks {
		ct.start(cm)
	}

	if cm.udpRTPListener != nil {
		cm.udpRTPListener.start(cm.c.state == clientStatePlay)
		cm.udpRTCPListener.start(cm.c.state == clientStatePlay)
	}

	for _, ct := range cm.tracks {
		ct.startWriting()
	}
}

func (cm *clientMedia) stop() {
	if cm.udpRTPListener != nil {
		cm.udpRTPListener.stop()
		cm.udpRTCPListener.stop()
	}

	for _, ct := range cm.tracks {
		ct.stop()
	}
}

func (cm *clientMedia) findTrackWithSSRC(ssrc uint32) *clientTrack {
	for _, track := range cm.tracks {
		tssrc, ok := track.udpRTCPReceiver.LastSSRC()
		if ok && tssrc == ssrc {
			return track
		}
	}
	return nil
}

func (cm *clientMedia) writePacketRTPInQueueUDP(payload []byte) {
	atomic.AddUint64(cm.c.BytesSent, uint64(len(payload)))
	cm.udpRTPListener.write(payload)
}

func (cm *clientMedia) writePacketRTCPInQueueUDP(payload []byte) {
	atomic.AddUint64(cm.c.BytesSent, uint64(len(payload)))
	cm.udpRTCPListener.write(payload)
}

func (cm *clientMedia) writePacketRTPInQueueTCP(payload []byte) {
	atomic.AddUint64(cm.c.BytesSent, uint64(len(payload)))
	cm.tcpRTPFrame.Payload = payload
	cm.c.nconn.SetWriteDeadline(time.Now().Add(cm.c.WriteTimeout))
	cm.c.conn.WriteInterleavedFrame(cm.tcpRTPFrame, cm.tcpBuffer)
}

func (cm *clientMedia) writePacketRTCPInQueueTCP(payload []byte) {
	atomic.AddUint64(cm.c.BytesSent, uint64(len(payload)))
	cm.tcpRTCPFrame.Payload = payload
	cm.c.nconn.SetWriteDeadline(time.Now().Add(cm.c.WriteTimeout))
	cm.c.conn.WriteInterleavedFrame(cm.tcpRTCPFrame, cm.tcpBuffer)
}

func (cm *clientMedia) writePacketRTCP(pkt rtcp.Packet) error {
	byts, err := pkt.Marshal()
	if err != nil {
		return err
	}

	cm.c.writeMutex.RLock()
	defer cm.c.writeMutex.RUnlock()

	ok := cm.c.writer.queue(func() {
		cm.writePacketRTCPInQueue(byts)
	})

	if !ok {
		select {
		case <-cm.c.done:
			return cm.c.closeError
		default:
			return nil
		}
	}

	return nil
}

func (cm *clientMedia) readRTPTCPPlay(payload []byte) error {
	now := time.Now()
	atomic.StoreInt64(cm.c.tcpLastFrameTime, now.Unix())

	pkt := cm.c.rtpPacketBuffer.next()
	err := pkt.Unmarshal(payload)
	if err != nil {
		return err
	}

	trak, ok := cm.tracks[pkt.PayloadType]
	if !ok {
		return nil
	}

	trak.readRTPTCP(pkt)
	return nil
}

func (cm *clientMedia) readRTCPTCPPlay(payload []byte) error {
	now := time.Now()
	atomic.StoreInt64(cm.c.tcpLastFrameTime, now.Unix())

	if len(payload) > maxPacketSize {
		cm.c.OnDecodeError(fmt.Errorf("RTCP packet size (%d) is greater than maximum allowed (%d)",
			len(payload), maxPacketSize))
		return nil
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return nil
	}

	for _, pkt := range packets {
		cm.onPacketRTCP(pkt)
	}

	return nil
}

func (cm *clientMedia) readRTPTCPRecord(payload []byte) error {
	return nil
}

func (cm *clientMedia) readRTCPTCPRecord(payload []byte) error {
	if len(payload) > maxPacketSize {
		cm.c.OnDecodeError(fmt.Errorf("RTCP packet size (%d) is greater than maximum allowed (%d)",
			len(payload), maxPacketSize))
		return nil
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return nil
	}

	for _, pkt := range packets {
		cm.onPacketRTCP(pkt)
	}

	return nil
}

func (cm *clientMedia) readRTPUDPPlay(payload []byte) error {
	plen := len(payload)

	atomic.AddUint64(cm.c.BytesReceived, uint64(plen))

	if plen == (maxPacketSize + 1) {
		cm.c.OnDecodeError(fmt.Errorf("RTP packet is too big to be read with UDP"))
		return nil
	}

	pkt := cm.c.rtpPacketBuffer.next()
	err := pkt.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return nil
	}

	trak, ok := cm.tracks[pkt.PayloadType]
	if !ok {
		cm.c.OnDecodeError(fmt.Errorf("received RTP packet with unknown payload type (%d)", pkt.PayloadType))
		return nil
	}

	trak.readRTPUDP(pkt)
	return nil
}

func (cm *clientMedia) readRTCPUDPPlay(payload []byte) error {
	now := time.Now()
	plen := len(payload)

	atomic.AddUint64(cm.c.BytesReceived, uint64(plen))

	if plen == (maxPacketSize + 1) {
		cm.c.OnDecodeError(fmt.Errorf("RTCP packet is too big to be read with UDP"))
		return nil
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return nil
	}

	for _, pkt := range packets {
		if sr, ok := pkt.(*rtcp.SenderReport); ok {
			track := cm.findTrackWithSSRC(sr.SSRC)
			if track != nil {
				track.udpRTCPReceiver.ProcessSenderReport(sr, now)
			}
		}

		cm.onPacketRTCP(pkt)
	}

	return nil
}

func (cm *clientMedia) readRTPUDPRecord(payload []byte) error {
	return nil
}

func (cm *clientMedia) readRTCPUDPRecord(payload []byte) error {
	plen := len(payload)

	atomic.AddUint64(cm.c.BytesReceived, uint64(plen))

	if plen == (maxPacketSize + 1) {
		cm.c.OnDecodeError(fmt.Errorf("RTCP packet is too big to be read with UDP"))
		return nil
	}

	packets, err := rtcp.Unmarshal(payload)
	if err != nil {
		cm.c.OnDecodeError(err)
		return nil
	}

	for _, pkt := range packets {
		cm.onPacketRTCP(pkt)
	}

	return nil
}
