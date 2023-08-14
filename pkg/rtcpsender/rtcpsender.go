// Package rtcpsender contains a utility to generate RTCP sender reports.
package rtcpsender

import (
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

var timeNow = time.Now

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
	writePacketRTCP func(rtcp.Packet)
	mutex           sync.Mutex

	// data from RTP packets
	initialized        bool
	lastTimeRTP        uint32
	lastTimeNTP        time.Time
	lastTimeSystem     time.Time
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
	period time.Duration,
	writePacketRTCP func(rtcp.Packet),
) *RTCPSender {
	rs := &RTCPSender{
		clockRate:       float64(clockRate),
		period:          period,
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

	if !rs.initialized {
		return nil
	}

	systemTimeDiff := timeNow().Sub(rs.lastTimeSystem)
	ntpTime := rs.lastTimeNTP.Add(systemTimeDiff)
	rtpTime := rs.lastTimeRTP + uint32(systemTimeDiff.Seconds()*rs.clockRate)

	return &rtcp.SenderReport{
		SSRC:        rs.lastSSRC,
		NTPTime:     ntpTimeGoToRTCP(ntpTime),
		RTPTime:     rtpTime,
		PacketCount: rs.packetCount,
		OctetCount:  rs.octetCount,
	}
}

// ProcessPacket extracts data from RTP packets.
func (rs *RTCPSender) ProcessPacket(pkt *rtp.Packet, ntp time.Time, ptsEqualsDTS bool) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if ptsEqualsDTS {
		rs.initialized = true
		rs.lastTimeRTP = pkt.Timestamp
		rs.lastTimeNTP = ntp
		rs.lastTimeSystem = timeNow()
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
