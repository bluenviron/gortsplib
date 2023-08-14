package gortsplib

import (
	"fmt"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats"
	"github.com/bluenviron/gortsplib/v3/pkg/rtcpreceiver"
	"github.com/bluenviron/gortsplib/v3/pkg/rtplossdetector"
	"github.com/bluenviron/gortsplib/v3/pkg/rtpreorderer"
)

type serverSessionFormat struct {
	sm              *serverSessionMedia
	format          formats.Format
	udpReorderer    *rtpreorderer.Reorderer
	tcpLossDetector *rtplossdetector.LossDetector
	udpRTCPReceiver *rtcpreceiver.RTCPReceiver
	onPacketRTP     OnPacketRTPFunc
}

func newServerSessionFormat(sm *serverSessionMedia, forma formats.Format) *serverSessionFormat {
	return &serverSessionFormat{
		sm:          sm,
		format:      forma,
		onPacketRTP: func(*rtp.Packet) {},
	}
}

func (sf *serverSessionFormat) start() {
	if sf.sm.ss.state != ServerSessionStatePlay {
		if *sf.sm.ss.setuppedTransport == TransportUDP || *sf.sm.ss.setuppedTransport == TransportUDPMulticast {
			sf.udpReorderer = rtpreorderer.New()
			var err error
			sf.udpRTCPReceiver, err = rtcpreceiver.New(
				sf.sm.ss.s.udpReceiverReportPeriod,
				nil,
				sf.format.ClockRate(),
				func(pkt rtcp.Packet) {
					sf.sm.ss.WritePacketRTCP(sf.sm.media, pkt) //nolint:errcheck
				})
			if err != nil {
				panic(err)
			}
		} else {
			sf.tcpLossDetector = rtplossdetector.New()
		}
	}
}

func (sf *serverSessionFormat) stop() {
	if sf.udpRTCPReceiver != nil {
		sf.udpRTCPReceiver.Close()
		sf.udpRTCPReceiver = nil
	}
}

func (sf *serverSessionFormat) readRTPUDP(pkt *rtp.Packet, now time.Time) {
	packets, lost := sf.udpReorderer.Process(pkt)
	if lost != 0 {
		sf.sm.ss.onPacketLost(fmt.Errorf("%d RTP %s lost",
			lost,
			func() string {
				if lost == 1 {
					return "packet"
				}
				return "packets"
			}()))
		// do not return
	}

	for _, pkt := range packets {
		sf.udpRTCPReceiver.ProcessPacket(pkt, now, sf.format.PTSEqualsDTS(pkt))
		sf.onPacketRTP(pkt)
	}
}

func (sf *serverSessionFormat) readRTPTCP(pkt *rtp.Packet) {
	lost := sf.tcpLossDetector.Process(pkt)
	if lost != 0 {
		sf.sm.ss.onPacketLost(fmt.Errorf("%d RTP %s lost",
			lost,
			func() string {
				if lost == 1 {
					return "packet"
				}
				return "packets"
			}()))
		// do not return
	}

	sf.onPacketRTP(pkt)
}
