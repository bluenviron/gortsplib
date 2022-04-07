// Package rtcpsender contains a utility to generate RTCP sender reports.
package rtcpsender

import (
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

var now = time.Now

// RTCPSender is a utility to generate RTCP sender reports.
type RTCPSender struct {
	period          time.Duration
	clockRate       float64
	writePacketRTCP func(rtcp.Packet)
	mutex           sync.Mutex

	// data from RTP packets
	firstRTPReceived bool
	senderSSRC       uint32
	lastRTPTimeRTP   uint32
	lastRTPTimeTime  time.Time
	packetCount      uint32
	octetCount       uint32

	terminate chan struct{}
	done      chan struct{}
}

// New allocates a RTCPSender.
func New(period time.Duration, clockRate int,
	writePacketRTCP func(rtcp.Packet),
) *RTCPSender {
	rs := &RTCPSender{
		period:          period,
		clockRate:       float64(clockRate),
		writePacketRTCP: writePacketRTCP,
		terminate:       make(chan struct{}),
		done:            make(chan struct{}),
	}

	go rs.run()

	return rs
}

// Close closes the RTCPSender.
func (rs *RTCPSender) Close() {
	close(rs.terminate)
	<-rs.done
}

func (rs *RTCPSender) run() {
	defer close(rs.done)

	t := time.NewTicker(rs.period)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			report := rs.report(now())
			if report != nil {
				rs.writePacketRTCP(report)
			}

		case <-rs.terminate:
			return
		}
	}
}

func (rs *RTCPSender) report(ts time.Time) rtcp.Packet {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if !rs.firstRTPReceived {
		return nil
	}

	return &rtcp.SenderReport{
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
}

// ProcessPacketRTP extracts the needed data from RTP packets.
func (rs *RTCPSender) ProcessPacketRTP(ts time.Time, pkt *rtp.Packet, ptsEqualsDTS bool) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if !rs.firstRTPReceived {
		rs.firstRTPReceived = true
		rs.senderSSRC = pkt.SSRC
	}

	if ptsEqualsDTS {
		rs.lastRTPTimeRTP = pkt.Timestamp
		rs.lastRTPTimeTime = ts
	}

	rs.packetCount++
	rs.octetCount += uint32(len(pkt.Payload))
}
