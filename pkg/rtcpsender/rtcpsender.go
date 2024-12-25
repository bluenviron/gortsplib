// Package rtcpsender contains a utility to generate RTCP sender reports.
package rtcpsender

import (
	"time"

	"github.com/bluenviron/gortsplib/v4/internal/rtcpsender"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

// RTCPSender is a utility to generate RTCP sender reports.
//
// Deprecated: will be removed in the next version.
type RTCPSender rtcpsender.RTCPSender

// New allocates a RTCPSender.
func New(
	clockRate int,
	period time.Duration,
	timeNow func() time.Time,
	writePacketRTCP func(rtcp.Packet),
) *RTCPSender {
	rs := &rtcpsender.RTCPSender{
		ClockRate:       clockRate,
		Period:          period,
		TimeNow:         timeNow,
		WritePacketRTCP: writePacketRTCP,
	}
	rs.Initialize()

	return (*RTCPSender)(rs)
}

// Close closes the RTCPSender.
func (rs *RTCPSender) Close() {
	(*rtcpsender.RTCPSender)(rs).Close()
}

// ProcessPacket extracts data from RTP packets.
func (rs *RTCPSender) ProcessPacket(pkt *rtp.Packet, ntp time.Time, ptsEqualsDTS bool) {
	(*rtcpsender.RTCPSender)(rs).ProcessPacket(pkt, ntp, ptsEqualsDTS)
}

// SenderSSRC returns the SSRC of outgoing RTP packets.
func (rs *RTCPSender) SenderSSRC() (uint32, bool) {
	return (*rtcpsender.RTCPSender)(rs).SenderSSRC()
}

// LastPacketData returns metadata of the last RTP packet.
func (rs *RTCPSender) LastPacketData() (uint16, uint32, time.Time, bool) {
	return (*rtcpsender.RTCPSender)(rs).LastPacketData()
}
