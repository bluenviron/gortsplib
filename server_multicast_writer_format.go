package gortsplib

import (
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/liberrors"
	"github.com/bluenviron/gortsplib/v5/pkg/rtpsender"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

type serverMulticastWriterFormat struct {
	senderReportPeriod       time.Duration
	timeNow                  func() time.Time
	disableRTCPSenderReports bool
	smm                      *serverMulticastWriterMedia
	format                   format.Format

	rtpSender *rtpsender.Sender
}

func (smf *serverMulticastWriterFormat) initialize() {
	smf.rtpSender = &rtpsender.Sender{
		ClockRate: smf.format.ClockRate(),
		Period:    smf.senderReportPeriod,
		TimeNow:   smf.timeNow,
		WritePacketRTCP: func(pkt rtcp.Packet) {
			if !smf.disableRTCPSenderReports {
				smf.smm.writePacketRTCP(pkt) //nolint:errcheck
			}
		},
	}
	smf.rtpSender.Initialize()
}

func (smf *serverMulticastWriterFormat) close() {
	smf.rtpSender.Close()
}

func (smf *serverMulticastWriterFormat) writePacketRTPEncoded(
	pkt *rtp.Packet,
	ntp time.Time,
	ptsEqualsDTS bool,
	payload []byte,
) error {
	smf.rtpSender.ProcessPacket(pkt, ntp, ptsEqualsDTS)

	ok := smf.smm.writer.Push(func() error {
		return smf.smm.rtpl.write(payload, smf.smm.rtpAddr)
	})
	if !ok {
		return liberrors.ErrServerWriteQueueFull{}
	}

	return nil
}
