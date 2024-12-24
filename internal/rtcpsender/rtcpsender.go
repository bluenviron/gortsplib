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
	ClockRate       int
	Period          time.Duration
	TimeNow         func() time.Time
	WritePacketRTCP func(rtcp.Packet)

	mutex sync.RWMutex

	// data from RTP packets
	initialized        bool
	lastTimeRTP        uint32
	lastTimeNTP        time.Time
	lastTimeSystem     time.Time
	senderSSRC         uint32
	lastSequenceNumber uint16
	packetCount        uint32
	octetCount         uint32

	terminate chan struct{}
	done      chan struct{}
}

// Initialize initializes a RTCPSender.
func (rs *RTCPSender) Initialize() {
	if rs.TimeNow == nil {
		rs.TimeNow = time.Now
	}

	rs.terminate = make(chan struct{})
	rs.done = make(chan struct{})

	go rs.run()
}

// Close closes the RTCPSender.
func (rs *RTCPSender) Close() {
	close(rs.terminate)
	<-rs.done
}

func (rs *RTCPSender) run() {
	defer close(rs.done)

	t := time.NewTicker(rs.Period)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			report := rs.report()
			if report != nil {
				rs.WritePacketRTCP(report)
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

	systemTimeDiff := rs.TimeNow().Sub(rs.lastTimeSystem)
	ntpTime := rs.lastTimeNTP.Add(systemTimeDiff)
	rtpTime := rs.lastTimeRTP + uint32(systemTimeDiff.Seconds()*float64(rs.ClockRate))

	return &rtcp.SenderReport{
		SSRC:        rs.senderSSRC,
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
		rs.lastTimeSystem = rs.TimeNow()
		rs.senderSSRC = pkt.SSRC
	}

	rs.lastSequenceNumber = pkt.SequenceNumber

	rs.packetCount++
	rs.octetCount += uint32(len(pkt.Payload))
}

// SenderSSRC returns the SSRC of outgoing RTP packets.
func (rs *RTCPSender) SenderSSRC() (uint32, bool) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	return rs.senderSSRC, rs.initialized
}

// LastPacketData returns metadata of the last RTP packet.
func (rs *RTCPSender) LastPacketData() (uint16, uint32, time.Time, bool) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	return rs.lastSequenceNumber, rs.lastTimeRTP, rs.lastTimeNTP, rs.initialized
}
