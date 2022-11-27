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
	clockRate       float64
	writePacketRTCP func(rtcp.Packet)
	mutex           sync.Mutex

	started bool
	period  time.Duration
	// data from RTP packets
	initialized        bool
	lastTimeRTP        uint32
	lastTimeNTP        time.Time
	lastSSRC           uint32
	lastSequenceNumber uint16
	packetCount        uint32
	octetCount         uint32

	terminate chan struct{}
	done      chan struct{}
}

// New allocates a RTCPSender.
func New(
	clockRate int,
	writePacketRTCP func(rtcp.Packet),
) *RTCPSender {
	rs := &RTCPSender{
		clockRate:       float64(clockRate),
		writePacketRTCP: writePacketRTCP,
		terminate:       make(chan struct{}),
		done:            make(chan struct{}),
	}

	return rs
}

// Close closes the RTCPSender.
func (rs *RTCPSender) Close() {
	if rs.started {
		close(rs.terminate)
		<-rs.done
	}
}

// Start starts the periodic generation of RTCP sender reports.
func (rs *RTCPSender) Start(period time.Duration) {
	rs.started = true
	rs.period = period
	go rs.run()
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

	if !rs.initialized || rs.clockRate == 0 {
		return nil
	}

	return &rtcp.SenderReport{
		SSRC: rs.lastSSRC,
		NTPTime: func() uint64 {
			// seconds since 1st January 1900
			// higher 32 bits are the integer part, lower 32 bits are the fractional part
			s := uint64(ts.UnixNano()) + 2208988800*1000000000
			return (s/1000000000)<<32 | (s % 1000000000)
		}(),
		RTPTime:     rs.lastTimeRTP + uint32((ts.Sub(rs.lastTimeNTP)).Seconds()*rs.clockRate),
		PacketCount: rs.packetCount,
		OctetCount:  rs.octetCount,
	}
}

// ProcessPacket extracts the needed data from RTP packets.
func (rs *RTCPSender) ProcessPacket(pkt *rtp.Packet, ntp time.Time, ptsEqualsDTS bool) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if ptsEqualsDTS {
		rs.initialized = true
		rs.lastTimeRTP = pkt.Timestamp
		rs.lastTimeNTP = ntp
	}

	rs.lastSSRC = pkt.SSRC
	rs.lastSequenceNumber = pkt.SequenceNumber

	rs.packetCount++
	rs.octetCount += uint32(len(pkt.Payload))
}

// LastSSRC returns the SSRC of the last RTP packet.
func (rs *RTCPSender) LastSSRC() (uint32, bool) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	return rs.lastSSRC, rs.initialized
}

// LastPacketData returns metadata of the last RTP packet.
func (rs *RTCPSender) LastPacketData() (uint16, uint32, time.Time, bool) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	return rs.lastSequenceNumber, rs.lastTimeRTP, rs.lastTimeNTP, rs.initialized
}
