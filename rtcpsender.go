package gortsplib

import (
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/rtp/v2"
)

type rtcpSender struct {
	period    time.Duration
	tracksLen int

	chain *interceptor.Chain
}

func newRTCPSender(period time.Duration, tracksLen int) *rtcpSender {
	rs := &rtcpSender{
		period:    period,
		tracksLen: tracksLen,
	}

	rs.chain = interceptor.NewChain(nil)

	return rs
}

func (rs *rtcpSender) close() {
}

func (rs *rtcpSender) processPacketRTP(now time.Time, trackID int, pkt *rtp.Packet) {
}
