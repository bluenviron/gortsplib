package gortsplib

import (
	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/internal/rtcpreceiver"
	"github.com/bluenviron/gortsplib/v4/internal/rtcpsender"
	"github.com/bluenviron/gortsplib/v4/internal/rtplossdetector"
	"github.com/bluenviron/gortsplib/v4/internal/rtpreorderer"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
)

type clientFormat struct {
	cm          *clientMedia
	format      format.Format
	onPacketRTP OnPacketRTPFunc

	udpReorderer    *rtpreorderer.Reorderer       // play
	tcpLossDetector *rtplossdetector.LossDetector // play
	rtcpReceiver    *rtcpreceiver.RTCPReceiver    // play
	rtcpSender      *rtcpsender.RTCPSender        // record or back channel
}

func (cf *clientFormat) start() {
	if cf.cm.c.state == clientStateRecord || cf.cm.media.IsBackChannel {
		cf.rtcpSender = &rtcpsender.RTCPSender{
			ClockRate: cf.format.ClockRate(),
			Period:    cf.cm.c.senderReportPeriod,
			TimeNow:   cf.cm.c.timeNow,
			WritePacketRTCP: func(pkt rtcp.Packet) {
				if !cf.cm.c.DisableRTCPSenderReports {
					cf.cm.c.WritePacketRTCP(cf.cm.media, pkt) //nolint:errcheck
				}
			},
		}
		cf.rtcpSender.Initialize()
	} else {
		if cf.cm.udpRTPListener != nil {
			cf.udpReorderer = &rtpreorderer.Reorderer{}
			cf.udpReorderer.Initialize()
		} else {
			cf.tcpLossDetector = &rtplossdetector.LossDetector{}
		}

		cf.rtcpReceiver = &rtcpreceiver.RTCPReceiver{
			ClockRate: cf.format.ClockRate(),
			Period:    cf.cm.c.receiverReportPeriod,
			TimeNow:   cf.cm.c.timeNow,
			WritePacketRTCP: func(pkt rtcp.Packet) {
				if cf.cm.udpRTPListener != nil {
					cf.cm.c.WritePacketRTCP(cf.cm.media, pkt) //nolint:errcheck
				}
			},
		}
		err := cf.rtcpReceiver.Initialize()
		if err != nil {
			panic(err)
		}
	}
}

func (cf *clientFormat) stop() {
	if cf.rtcpReceiver != nil {
		cf.rtcpReceiver.Close()
		cf.rtcpReceiver = nil
	}

	if cf.rtcpSender != nil {
		cf.rtcpSender.Close()
	}
}

func (cf *clientFormat) readRTPUDP(pkt *rtp.Packet) {
	packets, lost := cf.udpReorderer.Process(pkt)
	if lost != 0 {
		cf.cm.c.OnPacketLost(liberrors.ErrClientRTPPacketsLost{Lost: lost})
		// do not return
	}

	now := cf.cm.c.timeNow()

	for _, pkt := range packets {
		err := cf.rtcpReceiver.ProcessPacket(pkt, now, cf.format.PTSEqualsDTS(pkt))
		if err != nil {
			cf.cm.c.OnDecodeError(err)
			continue
		}

		cf.onPacketRTP(pkt)
	}
}

func (cf *clientFormat) readRTPTCP(pkt *rtp.Packet) {
	lost := cf.tcpLossDetector.Process(pkt)
	if lost != 0 {
		cf.cm.c.OnPacketLost(liberrors.ErrClientRTPPacketsLost{Lost: lost})
		// do not return
	}

	now := cf.cm.c.timeNow()

	err := cf.rtcpReceiver.ProcessPacket(pkt, now, cf.format.PTSEqualsDTS(pkt))
	if err != nil {
		cf.cm.c.OnDecodeError(err)
		return
	}

	cf.onPacketRTP(pkt)
}
