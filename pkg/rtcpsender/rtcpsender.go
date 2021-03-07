// Package rtcpsender contains a utility to generate RTCP sender reports.
package rtcpsender

import (
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/base"
)

// RTCPSender is a utility to generate RTCP sender reports.
type RTCPSender struct {
	clockRate float64
	mutex     sync.Mutex

	// data from rtp packets
	firstRTPReceived bool
	senderSSRC       uint32
	lastRTPTimeRTP   uint32
	lastRTPTimeTime  time.Time
	packetCount      uint32
	octetCount       uint32
}

// New allocates a RTCPSender.
func New(clockRate int) *RTCPSender {
	return &RTCPSender{
		clockRate: float64(clockRate),
	}
}

// ProcessFrame extracts the needed data from RTP or RTCP frames.
func (rs *RTCPSender) ProcessFrame(ts time.Time, streamType base.StreamType, buf []byte) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if streamType == base.StreamTypeRTP {
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(buf)
		if err == nil {
			if !rs.firstRTPReceived {
				rs.firstRTPReceived = true
				rs.senderSSRC = pkt.SSRC
			}

			// always update time to minimize errors
			rs.lastRTPTimeRTP = pkt.Timestamp
			rs.lastRTPTimeTime = ts

			rs.packetCount++
			rs.octetCount += uint32(len(pkt.Payload))
		}
	}
}

// Report generates a RTCP sender report.
// It returns nil if no packets has been passed to ProcessFrame yet.
func (rs *RTCPSender) Report(ts time.Time) []byte {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if !rs.firstRTPReceived {
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
		RTPTime:     rs.lastRTPTimeRTP + uint32((ts.Sub(rs.lastRTPTimeTime)).Seconds()*rs.clockRate),
		PacketCount: rs.packetCount,
		OctetCount:  rs.octetCount,
	}

	byts, err := report.Marshal()
	if err != nil {
		panic(err)
	}

	return byts
}
