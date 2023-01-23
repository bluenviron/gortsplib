package gortsplib

import (
	"fmt"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/rtcpreceiver"
	"github.com/aler9/gortsplib/v2/pkg/rtpreorderer"
)

type serverSessionFormat struct {
	sm              *serverSessionMedia
	format          format.Format
	udpReorderer    *rtpreorderer.Reorderer
	udpRTCPReceiver *rtcpreceiver.RTCPReceiver
	onPacketRTP     func(*rtp.Packet)
}

func newServerSessionFormat(sm *serverSessionMedia, forma format.Format) *serverSessionFormat {
	return &serverSessionFormat{
		sm:          sm,
		format:      forma,
		onPacketRTP: func(*rtp.Packet) {},
	}
}

func (sf *serverSessionFormat) start() {
	if (*sf.sm.ss.setuppedTransport == TransportUDP || *sf.sm.ss.setuppedTransport == TransportUDPMulticast) &&
		sf.sm.ss.state != ServerSessionStatePlay {
		sf.udpReorderer = rtpreorderer.New()
		sf.udpRTCPReceiver = rtcpreceiver.New(
			sf.sm.ss.s.udpReceiverReportPeriod,
			nil,
			sf.format.ClockRate(),
			func(pkt rtcp.Packet) {
				sf.sm.ss.WritePacketRTCP(sf.sm.media, pkt)
			})
	}
}

func (sf *serverSessionFormat) stop() {
	if sf.udpRTCPReceiver != nil {
		sf.udpRTCPReceiver.Close()
		sf.udpRTCPReceiver = nil
	}
}

func (sf *serverSessionFormat) readRTPUDP(pkt *rtp.Packet, now time.Time) {
	packets, missing := sf.udpReorderer.Process(pkt)
	if missing != 0 {
		onWarning(sf.sm.ss, fmt.Errorf("%d RTP packet(s) lost", missing))
		// do not return
	}

	for _, pkt := range packets {
		sf.udpRTCPReceiver.ProcessPacket(pkt, now, sf.format.PTSEqualsDTS(pkt))
		sf.onPacketRTP(pkt)
	}
}

func (sf *serverSessionFormat) readRTPTCP(pkt *rtp.Packet) {
	sf.onPacketRTP(pkt)
}
