package gortsplib

import (
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/liberrors"
	"github.com/bluenviron/gortsplib/v4/pkg/rtcpreceiver"
	"github.com/bluenviron/gortsplib/v4/pkg/rtcpsender"
	"github.com/bluenviron/gortsplib/v4/pkg/rtplossdetector"
	"github.com/bluenviron/gortsplib/v4/pkg/rtpreorderer"
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
		cf.rtcpSender = rtcpsender.New(
			cf.format.ClockRate(),
			cf.cm.c.senderReportPeriod,
			cf.cm.c.timeNow,
			func(pkt rtcp.Packet) {
				if !cf.cm.c.DisableRTCPSenderReports {
					cf.cm.c.WritePacketRTCP(cf.cm.media, pkt) //nolint:errcheck
				}
			})
	} else {
		if cf.cm.udpRTPListener != nil {
			cf.udpReorderer = rtpreorderer.New()
		} else {
			cf.tcpLossDetector = rtplossdetector.New()
		}

		var err error
		cf.rtcpReceiver, err = rtcpreceiver.New(
			cf.format.ClockRate(),
			nil,
			cf.cm.c.receiverReportPeriod,
			cf.cm.c.timeNow,
			func(pkt rtcp.Packet) {
				if cf.cm.udpRTPListener != nil {
					cf.cm.c.WritePacketRTCP(cf.cm.media, pkt) //nolint:errcheck
				}
			})
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

func (cf *clientFormat) writePacketRTP(byts []byte, pkt *rtp.Packet, ntp time.Time) error {
	cf.rtcpSender.ProcessPacket(pkt, ntp, cf.format.PTSEqualsDTS(pkt))

	ok := cf.cm.c.writer.push(func() {
		cf.cm.writePacketRTPInQueue(byts)
	})
	if !ok {
		return liberrors.ErrClientWriteQueueFull{}
	}

	return nil
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
