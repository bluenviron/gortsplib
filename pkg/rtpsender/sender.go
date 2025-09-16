// Package rtpsender contains a utility to send RTP packets.
package rtpsender

import (
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/ntp"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

// Sender is a utility to send RTP packets.
// It is in charge of generating RTCP sender reports.
type Sender struct {
	ClockRate       int
	Period          time.Duration
	TimeNow         func() time.Time
	WritePacketRTCP func(rtcp.Packet)

	mutex sync.RWMutex

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

// Initialize initializes a Sender.
func (rs *Sender) Initialize() {
	if rs.TimeNow == nil {
		rs.TimeNow = time.Now
	}

	rs.terminate = make(chan struct{})
	rs.done = make(chan struct{})

	go rs.run()
}

// Close closes the Sender.
func (rs *Sender) Close() {
	close(rs.terminate)
	<-rs.done
}

func (rs *Sender) run() {
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

func (rs *Sender) report() rtcp.Packet {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()

	if !rs.firstRTPPacketSent || rs.ClockRate == 0 {
		return nil
	}

	systemTimeDiff := rs.TimeNow().Sub(rs.lastTimeSystem)
	ntpTime := rs.lastTimeNTP.Add(systemTimeDiff)
	rtpTime := rs.lastTimeRTP + uint32(systemTimeDiff.Seconds()*float64(rs.ClockRate))

	return &rtcp.SenderReport{
		SSRC:        rs.localSSRC,
		NTPTime:     ntp.Encode(ntpTime),
		RTPTime:     rtpTime,
		PacketCount: rs.packetCount,
		OctetCount:  rs.octetCount,
	}
}

// ProcessPacket extracts data from RTP packets.
func (rs *Sender) ProcessPacket(pkt *rtp.Packet, ntp time.Time, ptsEqualsDTS bool) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if ptsEqualsDTS {
		rs.firstRTPPacketSent = true
		rs.lastTimeRTP = pkt.Timestamp
		rs.lastTimeNTP = ntp
		rs.lastTimeSystem = rs.TimeNow()
		rs.localSSRC = pkt.SSRC
	}

	rs.lastSequenceNumber = pkt.SequenceNumber

	rs.packetCount++
	rs.octetCount += uint32(len(pkt.Payload))
}

// Stats are statistics.
type Stats struct {
	LastSequenceNumber uint16
	LastRTP            uint32
	LastNTP            time.Time
}

// Stats returns statistics.
func (rs *Sender) Stats() *Stats {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()

	if !rs.firstRTPPacketSent {
		return nil
	}

	return &Stats{
		LastSequenceNumber: rs.lastSequenceNumber,
		LastRTP:            rs.lastTimeRTP,
		LastNTP:            rs.lastTimeNTP,
	}
}
