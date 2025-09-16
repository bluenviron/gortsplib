// Package rtpreceiver contains a utility to receive RTP packets.
package rtpreceiver

import (
	"fmt"
	"sync"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/ntp"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

// Receiver is a utility to receive RTP packets. It is in charge of:
// - removing packets with wrong SSRC
// - removing duplicate packets (when transport is unreliable)
// - reordering packets (when transport is unrealiable)
// - counting lost packets
// - generating RTCP receiver reports
type Receiver struct {
	// Track clock rate.
	ClockRate int

	// Local SSRC
	LocalSSRC uint32

	// Whether the transport is unrealiable.
	// This enables removing duplicate packets and reordering packets.
	UnrealiableTransport bool

	// size of the buffer for reordering packets.
	// It defaults to 64.
	BufferSize int

	// Period of RTCP receiver reports.
	Period time.Duration

	// time.Now function.
	TimeNow func() time.Time

	// Called when a RTCP receiver report is ready to be written.
	WritePacketRTCP func(rtcp.Packet)

	mutex sync.RWMutex

	// data from RTP packets
	firstRTPPacketReceived bool
	timeInitialized        bool
	buffer                 []*rtp.Packet
	absPos                 uint16
	negativeCount          int
	sequenceNumberCycles   uint16
	lastValidSeqNum        uint16
	remoteSSRC             uint32
	lastTimeRTP            uint32
	lastTimeSystem         time.Time
	totalLost              uint32
	totalLostSinceReport   uint32
	totalSinceReport       uint32
	jitter                 float64

	// data from RTCP packets
	firstSenderReportReceived  bool
	lastSenderReportTimeNTP    uint64
	lastSenderReportTimeRTP    uint32
	lastSenderReportTimeSystem time.Time

	terminate chan struct{}
	done      chan struct{}
}

// Initialize initializes Receiver.
func (rr *Receiver) Initialize() error {
	if rr.BufferSize == 0 {
		rr.BufferSize = 64
	}

	if rr.Period == 0 {
		return fmt.Errorf("invalid Period")
	}

	if rr.TimeNow == nil {
		rr.TimeNow = time.Now
	}

	if rr.UnrealiableTransport {
		rr.buffer = make([]*rtp.Packet, rr.BufferSize)
	}

	rr.terminate = make(chan struct{})
	rr.done = make(chan struct{})

	go rr.run()

	return nil
}

// Close closes the Receiver.
func (rr *Receiver) Close() {
	close(rr.terminate)
	<-rr.done
}

func (rr *Receiver) run() {
	defer close(rr.done)

	t := time.NewTicker(rr.Period)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			report := rr.report()
			if report != nil {
				rr.WritePacketRTCP(report)
			}

		case <-rr.terminate:
			return
		}
	}
}

func (rr *Receiver) report() rtcp.Packet {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	if !rr.firstRTPPacketReceived || rr.ClockRate == 0 {
		return nil
	}

	system := rr.TimeNow()

	report := &rtcp.ReceiverReport{
		SSRC: rr.LocalSSRC,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               rr.remoteSSRC,
				LastSequenceNumber: uint32(rr.sequenceNumberCycles)<<16 | uint32(rr.lastValidSeqNum),
				// equivalent to taking the integer part after multiplying the
				// loss fraction by 256
				FractionLost: uint8(float64(rr.totalLostSinceReport*256) / float64(rr.totalSinceReport)),
				TotalLost:    rr.totalLost,
				Jitter:       uint32(rr.jitter),
			},
		},
	}

	if rr.firstSenderReportReceived {
		// middle 32 bits out of 64 in the NTP of last sender report
		report.Reports[0].LastSenderReport = uint32(rr.lastSenderReportTimeNTP >> 16)

		// delay, expressed in units of 1/65536 seconds, between
		// receiving the last SR packet from source SSRC_n and sending this
		// reception report block
		report.Reports[0].Delay = uint32(system.Sub(rr.lastSenderReportTimeSystem).Seconds() * 65536)
	}

	rr.totalLostSinceReport = 0
	rr.totalSinceReport = 0

	return report
}

// ProcessPacket processes an incoming RTP packet.
// It returns reordered packets and number of lost packets.
func (rr *Receiver) ProcessPacket(
	pkt *rtp.Packet,
	system time.Time,
	ptsEqualsDTS bool,
) ([]*rtp.Packet, uint64, error) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	// first packet
	if !rr.firstRTPPacketReceived {
		rr.firstRTPPacketReceived = true
		rr.totalSinceReport = 1
		rr.lastValidSeqNum = pkt.SequenceNumber
		rr.remoteSSRC = pkt.SSRC

		if ptsEqualsDTS {
			rr.timeInitialized = true
			rr.lastTimeRTP = pkt.Timestamp
			rr.lastTimeSystem = system
		}

		return []*rtp.Packet{pkt}, 0, nil
	}

	if pkt.SSRC != rr.remoteSSRC {
		return nil, 0, fmt.Errorf("received packet with wrong SSRC %d, expected %d", pkt.SSRC, rr.remoteSSRC)
	}

	var pkts []*rtp.Packet
	var lost uint64

	if rr.UnrealiableTransport {
		pkts, lost = rr.reorder(pkt)
	} else {
		pkts = []*rtp.Packet{pkt}
		lost = uint64(pkt.SequenceNumber - rr.lastValidSeqNum - 1)
	}

	rr.totalLost += uint32(lost)
	rr.totalLostSinceReport += uint32(lost)

	// allow up to 24 bits
	if rr.totalLost > 0xFFFFFF {
		rr.totalLost = 0xFFFFFF
	}
	if rr.totalLostSinceReport > 0xFFFFFF {
		rr.totalLostSinceReport = 0xFFFFFF
	}

	for _, pkt := range pkts {
		diff := int32(pkt.SequenceNumber) - int32(rr.lastValidSeqNum)

		// overflow
		if diff < -0x0FFF {
			rr.sequenceNumberCycles++
		}

		rr.totalSinceReport += uint32(uint16(diff))
		rr.lastValidSeqNum = pkt.SequenceNumber

		if ptsEqualsDTS {
			if rr.timeInitialized && rr.ClockRate != 0 {
				// update jitter
				// https://tools.ietf.org/html/rfc3550#page-39
				D := system.Sub(rr.lastTimeSystem).Seconds()*float64(rr.ClockRate) -
					(float64(pkt.Timestamp) - float64(rr.lastTimeRTP))
				if D < 0 {
					D = -D
				}
				rr.jitter += (D - rr.jitter) / 16
			}

			rr.timeInitialized = true
			rr.lastTimeRTP = pkt.Timestamp
			rr.lastTimeSystem = system
		}
	}

	return pkts, lost, nil
}

func (rr *Receiver) reorder(pkt *rtp.Packet) ([]*rtp.Packet, uint64) {
	relPos := int16(pkt.SequenceNumber - rr.lastValidSeqNum - 1) // rr.expectedSeqNum)

	// packet is a duplicate or has been sent
	// before the first packet processed by Reorderer.
	// discard.
	if relPos < 0 {
		rr.negativeCount++

		// stream has been resetted, therefore reset reorderer too
		if rr.negativeCount > len(rr.buffer) {
			rr.negativeCount = 0

			// clear buffer
			for i := uint16(0); i < uint16(len(rr.buffer)); i++ {
				p := (rr.absPos + i) & (uint16(len(rr.buffer)) - 1)
				rr.buffer[p] = nil
			}

			// reset position.
			return []*rtp.Packet{pkt}, 0
		}

		return nil, 0
	}

	rr.negativeCount = 0

	// there's a missing packet and buffer is full.
	// return entire buffer and clear it.
	if relPos >= int16(len(rr.buffer)) {
		n := 1
		for i := uint16(0); i < uint16(len(rr.buffer)); i++ {
			p := (rr.absPos + i) & (uint16(len(rr.buffer)) - 1)
			if rr.buffer[p] != nil {
				n++
			}
		}

		ret := make([]*rtp.Packet, n)
		pos := 0

		for i := uint16(0); i < uint16(len(rr.buffer)); i++ {
			p := (rr.absPos + i) & (uint16(len(rr.buffer)) - 1)
			if rr.buffer[p] != nil {
				ret[pos], rr.buffer[p] = rr.buffer[p], nil
				pos++
			}
		}

		ret[pos] = pkt

		return ret, uint64(int(relPos) - n + 1)
	}

	// there's a missing packet
	if relPos != 0 {
		p := (rr.absPos + uint16(relPos)) & (uint16(len(rr.buffer)) - 1)

		// current packet is a duplicate. discard
		if rr.buffer[p] != nil {
			return nil, 0
		}

		// put current packet in buffer
		rr.buffer[p] = pkt
		return nil, 0
	}

	// all packets have been received correctly.
	// return them.

	n := uint16(1)
	for {
		p := (rr.absPos + n) & (uint16(len(rr.buffer)) - 1)
		if rr.buffer[p] == nil {
			break
		}
		n++
	}

	ret := make([]*rtp.Packet, n)
	ret[0] = pkt
	rr.absPos++
	rr.absPos &= (uint16(len(rr.buffer)) - 1)

	for i := uint16(1); i < n; i++ {
		ret[i], rr.buffer[rr.absPos] = rr.buffer[rr.absPos], nil
		rr.absPos++
		rr.absPos &= (uint16(len(rr.buffer)) - 1)
	}

	return ret, 0
}

// ProcessSenderReport processes an incoming RTCP sender report.
func (rr *Receiver) ProcessSenderReport(sr *rtcp.SenderReport, system time.Time) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	rr.firstSenderReportReceived = true
	rr.lastSenderReportTimeNTP = sr.NTPTime
	rr.lastSenderReportTimeRTP = sr.RTPTime
	rr.lastSenderReportTimeSystem = system
}

func (rr *Receiver) packetNTPUnsafe(ts uint32) (time.Time, bool) {
	if !rr.firstSenderReportReceived || rr.ClockRate == 0 {
		return time.Time{}, false
	}

	timeDiff := int32(ts - rr.lastSenderReportTimeRTP)
	timeDiffGo := (time.Duration(timeDiff) * time.Second) / time.Duration(rr.ClockRate)

	return ntp.Decode(rr.lastSenderReportTimeNTP).Add(timeDiffGo), true
}

// PacketNTP returns the NTP (absolute timestamp) of the packet.
func (rr *Receiver) PacketNTP(ts uint32) (time.Time, bool) {
	rr.mutex.RLock()
	defer rr.mutex.RUnlock()

	return rr.packetNTPUnsafe(ts)
}

// Stats are statistics.
type Stats struct {
	RemoteSSRC         uint32
	LastSequenceNumber uint16
	LastRTP            uint32
	LastNTP            time.Time
	Jitter             float64
}

// Stats returns statistics.
func (rr *Receiver) Stats() *Stats {
	rr.mutex.RLock()
	defer rr.mutex.RUnlock()

	if !rr.firstRTPPacketReceived {
		return nil
	}

	ntp, _ := rr.packetNTPUnsafe(rr.lastTimeRTP)

	return &Stats{
		RemoteSSRC:         rr.remoteSSRC,
		LastSequenceNumber: rr.lastValidSeqNum,
		LastRTP:            rr.lastTimeRTP,
		LastNTP:            ntp,
		Jitter:             rr.jitter,
	}
}
