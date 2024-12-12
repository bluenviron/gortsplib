// Package rtcpreceiver contains a utility to generate RTCP receiver reports.
package rtcpreceiver

import (
	"time"

	"github.com/bluenviron/gortsplib/v4/internal/rtcpreceiver"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

// RTCPReceiver is a utility to generate RTCP receiver reports.
//
// Deprecated: will be removed in the next version.
type RTCPReceiver rtcpreceiver.RTCPReceiver

// New allocates a RTCPReceiver.
func New(
	clockRate int,
	receiverSSRC *uint32,
	period time.Duration,
	timeNow func() time.Time,
	writePacketRTCP func(rtcp.Packet),
) (*RTCPReceiver, error) {
	rr := &rtcpreceiver.RTCPReceiver{
		ClockRate:       clockRate,
		LocalSSRC:       receiverSSRC,
		Period:          period,
		TimeNow:         timeNow,
		WritePacketRTCP: writePacketRTCP,
	}
	err := rr.Initialize()
	if err != nil {
		return nil, err
	}

	return (*RTCPReceiver)(rr), nil
}

// Close closes the RTCPReceiver.
func (rr *RTCPReceiver) Close() {
	(*rtcpreceiver.RTCPReceiver)(rr).Close()
}

// ProcessPacket extracts the needed data from RTP packets.
func (rr *RTCPReceiver) ProcessPacket(pkt *rtp.Packet, system time.Time, ptsEqualsDTS bool) error {
	return (*rtcpreceiver.RTCPReceiver)(rr).ProcessPacketRTP(pkt, system, ptsEqualsDTS)
}

// ProcessSenderReport extracts the needed data from RTCP sender reports.
func (rr *RTCPReceiver) ProcessSenderReport(sr *rtcp.SenderReport, system time.Time) {
	(*rtcpreceiver.RTCPReceiver)(rr).ProcessSenderReport(sr, system)
}

// PacketNTP returns the NTP timestamp of the packet.
func (rr *RTCPReceiver) PacketNTP(ts uint32) (time.Time, bool) {
	return (*rtcpreceiver.RTCPReceiver)(rr).PacketNTP(ts)
}

// SenderSSRC returns the SSRC of outgoing RTP packets.
func (rr *RTCPReceiver) SenderSSRC() (uint32, bool) {
	stats := (*rtcpreceiver.RTCPReceiver)(rr).Stats()
	if stats == nil {
		return 0, false
	}
	return stats.RemoteSSRC, true
}
