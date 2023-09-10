package gortsplib

import (
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
	"github.com/bluenviron/gortsplib/v4/pkg/rtcpreceiver"
	"github.com/bluenviron/gortsplib/v4/pkg/rtplossdetector"
	"github.com/bluenviron/gortsplib/v4/pkg/rtpreorderer"
)

type serverSessionFormat struct {
	sm              *serverSessionMedia
	format          format.Format
	udpReorderer    *rtpreorderer.Reorderer
	tcpLossDetector *rtplossdetector.LossDetector
	rtcpReceiver    *rtcpreceiver.RTCPReceiver
	onPacketRTP     OnPacketRTPFunc
}

func newServerSessionFormat(sm *serverSessionMedia, forma format.Format) *serverSessionFormat {
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
		} else {
			sf.tcpLossDetector = rtplossdetector.New()
		}

		var err error
		sf.rtcpReceiver, err = rtcpreceiver.New(
			sf.format.ClockRate(),
			nil,
			sf.sm.ss.s.receiverReportPeriod,
			sf.sm.ss.s.timeNow,
			func(pkt rtcp.Packet) {
				if *sf.sm.ss.setuppedTransport == TransportUDP || *sf.sm.ss.setuppedTransport == TransportUDPMulticast {
					sf.sm.ss.WritePacketRTCP(sf.sm.media, pkt) //nolint:errcheck
				}
			})
		if err != nil {
			panic(err)
		}
	}
}

func (sf *serverSessionFormat) stop() {
	if sf.rtcpReceiver != nil {
		sf.rtcpReceiver.Close()
		sf.rtcpReceiver = nil
	}
}

func (sf *serverSessionFormat) readRTPUDP(pkt *rtp.Packet, now time.Time) {
	packets, lost := sf.udpReorderer.Process(pkt)
	if lost != 0 {
		sf.sm.ss.onPacketLost(liberrors.ErrServerRTPPacketsLost{Lost: lost})
		// do not return
	}

	for _, pkt := range packets {
		err := sf.rtcpReceiver.ProcessPacket(pkt, now, sf.format.PTSEqualsDTS(pkt))
		if err != nil {
			sf.sm.ss.onDecodeError(err)
			continue
		}

		sf.onPacketRTP(pkt)
	}
}

func (sf *serverSessionFormat) readRTPTCP(pkt *rtp.Packet) {
	lost := sf.tcpLossDetector.Process(pkt)
	if lost != 0 {
		sf.sm.ss.onPacketLost(liberrors.ErrServerRTPPacketsLost{Lost: lost})
		// do not return
	}

	now := sf.sm.ss.s.timeNow()

	err := sf.rtcpReceiver.ProcessPacket(pkt, now, sf.format.PTSEqualsDTS(pkt))
	if err != nil {
		sf.sm.ss.onDecodeError(err)
		return
	}

	sf.onPacketRTP(pkt)
}
