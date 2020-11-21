// Package rtcpsender implements a utility to generate RTCP sender reports.
package rtcpsender

import (
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/base"
)

// RtcpSender allows to generate RTCP sender reports.
type RtcpSender struct {
	mutex             sync.Mutex
	firstRtpReceived  bool
	secondRtpReceived bool
	senderSSRC        uint32
	packetCount       uint32
	octetCount        uint32
	rtpTimeOffset     uint32
	rtpTimeTime       time.Time
	clock             float64
}

// New allocates a RtcpSender.
func New() *RtcpSender {
	return &RtcpSender{}
}

// OnFrame processes a RTP or RTCP frame and extract the needed data.
func (rs *RtcpSender) OnFrame(ts time.Time, streamType base.StreamType, buf []byte) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if streamType == base.StreamTypeRtp {
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(buf)
		if err == nil {
			if !rs.firstRtpReceived {
				rs.firstRtpReceived = true
				rs.senderSSRC = pkt.SSRC

				// save RTP time offset and correspondent time
				rs.rtpTimeOffset = pkt.Timestamp
				rs.rtpTimeTime = ts

			} else if !rs.secondRtpReceived && pkt.Timestamp != rs.rtpTimeOffset {
				rs.secondRtpReceived = true

				// estimate clock
				rs.clock = float64(pkt.Timestamp-rs.rtpTimeOffset) /
					ts.Sub(rs.rtpTimeTime).Seconds()
			}

			rs.packetCount++
			rs.octetCount += uint32(len(pkt.Payload))
		}
	}
}

// Report generates a RTCP sender report.
func (rs *RtcpSender) Report(ts time.Time) []byte {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if !rs.firstRtpReceived || !rs.secondRtpReceived {
		return nil
	}

	report := &rtcp.SenderReport{
		SSRC: rs.senderSSRC,
		NTPTime: func() uint64 {
			// seconds since 1st January 1900
			n := (float64(ts.UnixNano()) / 1000000000) + 2208988800

			// higher 32 bits are the integer part, lower 32 bits are the fractional part
			integerPart := uint32(n)
			fractionalPart := uint32((n - float64(integerPart)) * 0xFFFFFFFF)
			return uint64(integerPart)<<32 | uint64(fractionalPart)
		}(),
		RTPTime:     rs.rtpTimeOffset + uint32((ts.Sub(rs.rtpTimeTime)).Seconds()*rs.clock),
		PacketCount: rs.packetCount,
		OctetCount:  rs.octetCount,
	}

	byts, _ := report.Marshal()
	return byts
}
