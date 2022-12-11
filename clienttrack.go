package gortsplib

import (
	"fmt"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/rtcpreceiver"
	"github.com/aler9/gortsplib/v2/pkg/rtcpsender"
	"github.com/aler9/gortsplib/v2/pkg/rtpreorderer"
)

type clientFormat struct {
	c               *Client
	cm              *clientMedia
	format          format.Format
	udpReorderer    *rtpreorderer.Reorderer    // play
	udpRTCPReceiver *rtcpreceiver.RTCPReceiver // play
	rtcpSender      *rtcpsender.RTCPSender     // record
	onPacketRTP     func(*rtp.Packet)
}

func newClientFormat(cm *clientMedia, trak format.Format) *clientFormat {
	return &clientFormat{
		c:           cm.c,
		cm:          cm,
		format:      trak,
		onPacketRTP: func(*rtp.Packet) {},
	}
}

func (ct *clientFormat) start(cm *clientMedia) {
	if cm.c.state == clientStatePlay {
		if cm.udpRTPListener != nil {
			ct.udpReorderer = rtpreorderer.New()
			ct.udpRTCPReceiver = rtcpreceiver.New(
				cm.c.udpReceiverReportPeriod,
				nil,
				ct.format.ClockRate(), func(pkt rtcp.Packet) {
					cm.writePacketRTCP(pkt)
				})
		}
	} else {
		ct.rtcpSender = rtcpsender.New(
			ct.format.ClockRate(),
			func(pkt rtcp.Packet) {
				cm.writePacketRTCP(pkt)
			})
	}
}

// start RTCP senders after write() has been allocated in order to avoid a crash
func (ct *clientFormat) startWriting() {
	if ct.c.state != clientStatePlay && !ct.c.DisableRTCPSenderReports {
		ct.rtcpSender.Start(ct.c.senderReportPeriod)
	}
}

func (ct *clientFormat) stop() {
	if ct.udpRTCPReceiver != nil {
		ct.udpRTCPReceiver.Close()
		ct.udpRTCPReceiver = nil
	}

	if ct.rtcpSender != nil {
		ct.rtcpSender.Close()
		ct.rtcpSender = nil
	}
}

func (ct *clientFormat) writePacketRTPWithNTP(pkt *rtp.Packet, ntp time.Time) error {
	byts := make([]byte, maxPacketSize)
	n, err := pkt.MarshalTo(byts)
	if err != nil {
		return err
	}
	byts = byts[:n]

	ct.c.writeMutex.RLock()
	defer ct.c.writeMutex.RUnlock()

	ok := ct.c.writer.queue(func() {
		ct.cm.writePacketRTPInQueue(byts)
	})

	if !ok {
		select {
		case <-ct.c.done:
			return ct.c.closeError
		default:
			return nil
		}
	}

	ct.rtcpSender.ProcessPacket(pkt, ntp, ct.format.PTSEqualsDTS(pkt))
	return nil
}

func (ct *clientFormat) readRTPUDP(pkt *rtp.Packet) {
	packets, missing := ct.udpReorderer.Process(pkt)
	if missing != 0 {
		ct.c.OnDecodeError(fmt.Errorf("%d RTP packet(s) lost", missing))
		// do not return
	}

	now := time.Now()

	for _, pkt := range packets {
		ct.udpRTCPReceiver.ProcessPacket(pkt, now, ct.format.PTSEqualsDTS(pkt))
		ct.onPacketRTP(pkt)
	}
}

func (ct *clientFormat) readRTPTCP(pkt *rtp.Packet) {
	ct.onPacketRTP(pkt)
}
