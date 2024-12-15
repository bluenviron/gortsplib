// Package rtcpsender contains a utility to generate RTCP sender reports.
package rtcpsender

import (
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

// seconds since 1st January 1900
// higher 32 bits are the integer part, lower 32 bits are the fractional part
func ntpTimeGoToRTCP(v time.Time) uint64 {
	s := uint64(v.UnixNano()) + 2208988800*1000000000
	return (s/1000000000)<<32 | (s % 1000000000)
}

// RTCPSender is a utility to generate RTCP sender reports.
type RTCPSender struct {
	clockRate       float64
	period          time.Duration
	timeNow         func() time.Time
	writePacketRTCP func(rtcp.Packet)
	mutex           sync.RWMutex

	// data from RTP packets
	firstRTPPacketSent bool
	lastTimeRTP        uint32
	lastTimeNTP        time.Time
	lastTimeSystem     time.Time
	localSSRC          uint32
	lastSequenceNumber uint16
	packetCount        uint32
	octetCount         uint32

	terminate chan struct{}
	done      chan struct{}
}

// New allocates a RTCPSender.
func New(
	clockRate int,
	period time.Duration,
	timeNow func() time.Time,
	writePacketRTCP func(rtcp.Packet),
) *RTCPSender {
	if timeNow == nil {
		timeNow = time.Now
	}

	rs := &RTCPSender{
		clockRate:       float64(clockRate),
		period:          period,
		timeNow:         timeNow,
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
			report := rs.report()
			if report != nil {
				rs.writePacketRTCP(report)
			}

		case <-rs.terminate:
			return
		}
	}
}

func (rs *RTCPSender) report() rtcp.Packet {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if !rs.firstRTPPacketSent {
		return nil
	}

	systemTimeDiff := rs.timeNow().Sub(rs.lastTimeSystem)
	ntpTime := rs.lastTimeNTP.Add(systemTimeDiff)
	rtpTime := rs.lastTimeRTP + uint32(systemTimeDiff.Seconds()*rs.clockRate)

	return &rtcp.SenderReport{
		SSRC:        rs.localSSRC,
		NTPTime:     ntpTimeGoToRTCP(ntpTime),
		RTPTime:     rtpTime,
		PacketCount: rs.packetCount,
		OctetCount:  rs.octetCount,
	}
}

// ProcessRTPPacket extracts required data from RTP packets.
func (rs *RTCPSender) ProcessRTPPacket(pkt *rtp.Packet, ntp time.Time, ptsEqualsDTS bool) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if ptsEqualsDTS {
		rs.firstRTPPacketSent = true
		rs.lastTimeRTP = pkt.Timestamp
		rs.lastTimeNTP = ntp
		rs.lastTimeSystem = rs.timeNow()
		rs.localSSRC = pkt.SSRC
	}

	rs.lastSequenceNumber = pkt.SequenceNumber

	rs.packetCount++
	rs.octetCount += uint32(len(pkt.Payload))
}

// Stats are statistics.
type Stats struct {
	LocalSSRC          uint32
	LastSequenceNumber uint16
	LastTimeRTP        uint32
	LastTimeNTP        time.Time
}

// Stats returns statistics.
func (rs *RTCPSender) Stats() *Stats {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()

	if !rs.firstRTPPacketSent {
		return nil
	}

	return &Stats{
		LocalSSRC:          rs.localSSRC,
		LastSequenceNumber: rs.lastSequenceNumber,
		LastTimeRTP:        rs.lastTimeRTP,
		LastTimeNTP:        rs.lastTimeNTP,
	}
}
