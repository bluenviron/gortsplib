// Package rtcpsender implements a utility to generate RTCP sender reports.
package rtcpsender

import (
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/base"
)

// RtcpSender is a utility to generate RTCP sender reports.
type RtcpSender struct {
	clockRate float64
	mutex     sync.Mutex

	// data from rtp packets
	firstRtpReceived bool
	senderSSRC       uint32
	lastRtpTimeRtp   uint32
	lastRtpTimeTime  time.Time
	packetCount      uint32
	octetCount       uint32
}

// New allocates a RtcpSender.
func New(clockRate int) *RtcpSender {
	return &RtcpSender{
		clockRate: float64(clockRate),
	}
}

// ProcessFrame extracts the needed data from RTP or RTCP frames.
func (rs *RtcpSender) ProcessFrame(ts time.Time, streamType base.StreamType, buf []byte) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if streamType == base.StreamTypeRtp {
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(buf)
		if err == nil {
			if !rs.firstRtpReceived {
				rs.firstRtpReceived = true
				rs.senderSSRC = pkt.SSRC
			}

			// always update time to minimize errors
			rs.lastRtpTimeRtp = pkt.Timestamp
			rs.lastRtpTimeTime = ts

			rs.packetCount++
			rs.octetCount += uint32(len(pkt.Payload))
		}
	}
}

// Report generates a RTCP sender report.
// It returns nil if no packets has been passed to ProcessFrame yet.
func (rs *RtcpSender) Report(ts time.Time) []byte {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if !rs.firstRtpReceived {
		return nil
	}

	report := &rtcp.SenderReport{
		SSRC: rs.senderSSRC,
		NTPTime: func() uint64 {
			// seconds since 1st January 1900
			s := (float64(ts.UnixNano()) / 1000000000) + 2208988800

			// higher 32 bits are the integer part, lower 32 bits are the fractional part
			integerPart := uint32(s)
			fractionalPart := uint32((s - float64(integerPart)) * 0xFFFFFFFF)
			return uint64(integerPart)<<32 | uint64(fractionalPart)
		}(),
		RTPTime:     rs.lastRtpTimeRtp + uint32((ts.Sub(rs.lastRtpTimeTime)).Seconds()*float64(rs.clockRate)),
		PacketCount: rs.packetCount,
		OctetCount:  rs.octetCount,
	}

	byts, err := report.Marshal()
	if err != nil {
		panic(err)
	}

	return byts
}
