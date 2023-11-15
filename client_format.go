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
	cm              *clientMedia
	format          format.Format
	udpReorderer    *rtpreorderer.Reorderer       // play
	tcpLossDetector *rtplossdetector.LossDetector // play
	rtcpReceiver    *rtcpreceiver.RTCPReceiver    // play
	rtcpSender      *rtcpsender.RTCPSender        // record or back channel
	onPacketRTP     OnPacketRTPFunc
}

func newClientFormat(cm *clientMedia, forma format.Format) *clientFormat {
	return &clientFormat{
		cm:          cm,
		format:      forma,
		onPacketRTP: func(*rtp.Packet) {},
	}
}

func (ct *clientFormat) start() {
	if ct.cm.c.state == clientStateRecord || ct.cm.media.IsBackChannel {
		ct.rtcpSender = rtcpsender.New(
			ct.format.ClockRate(),
			ct.cm.c.senderReportPeriod,
			ct.cm.c.timeNow,
			func(pkt rtcp.Packet) {
				if !ct.cm.c.DisableRTCPSenderReports {
					ct.cm.c.WritePacketRTCP(ct.cm.media, pkt) //nolint:errcheck
				}
			})
	} else {
		if ct.cm.udpRTPListener != nil {
			ct.udpReorderer = rtpreorderer.New()
		} else {
			ct.tcpLossDetector = rtplossdetector.New()
		}

		var err error
		ct.rtcpReceiver, err = rtcpreceiver.New(
			ct.format.ClockRate(),
			nil,
			ct.cm.c.receiverReportPeriod,
			ct.cm.c.timeNow,
			func(pkt rtcp.Packet) {
				if ct.cm.udpRTPListener != nil {
					ct.cm.c.WritePacketRTCP(ct.cm.media, pkt) //nolint:errcheck
				}
			})
		if err != nil {
			panic(err)
		}
	}
}

func (ct *clientFormat) stop() {
	if ct.rtcpReceiver != nil {
		ct.rtcpReceiver.Close()
		ct.rtcpReceiver = nil
	}

	if ct.rtcpSender != nil {
		ct.rtcpSender.Close()
	}
}

func (ct *clientFormat) writePacketRTP(byts []byte, pkt *rtp.Packet, ntp time.Time) error {
	ct.rtcpSender.ProcessPacket(pkt, ntp, ct.format.PTSEqualsDTS(pkt))

	ok := ct.cm.c.writer.push(func() {
		ct.cm.writePacketRTPInQueue(byts)
	})
	if !ok {
		return liberrors.ErrClientWriteQueueFull{}
	}

	return nil
}

func (ct *clientFormat) readRTPUDP(pkt *rtp.Packet) {
	packets, lost := ct.udpReorderer.Process(pkt)
	if lost != 0 {
		ct.cm.c.OnPacketLost(liberrors.ErrClientRTPPacketsLost{Lost: lost})
		// do not return
	}

	now := ct.cm.c.timeNow()

	for _, pkt := range packets {
		err := ct.rtcpReceiver.ProcessPacket(pkt, now, ct.format.PTSEqualsDTS(pkt))
		if err != nil {
			ct.cm.c.OnDecodeError(err)
			continue
		}

		ct.onPacketRTP(pkt)
	}
}

func (ct *clientFormat) readRTPTCP(pkt *rtp.Packet) {
	lost := ct.tcpLossDetector.Process(pkt)
	if lost != 0 {
		ct.cm.c.OnPacketLost(liberrors.ErrClientRTPPacketsLost{Lost: lost})
		// do not return
	}

	now := ct.cm.c.timeNow()

	err := ct.rtcpReceiver.ProcessPacket(pkt, now, ct.format.PTSEqualsDTS(pkt))
	if err != nil {
		ct.cm.c.OnDecodeError(err)
		return
	}

	ct.onPacketRTP(pkt)
}
