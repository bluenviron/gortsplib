// Package rtpsender contains a utility to send RTP packets.
package rtpsender

import (
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v5/pkg/ntp"
)

// Sender is a utility to send RTP packets.
// It is in charge of:
// - counting sent packets
// - generating RTCP sender reports.
// - parsing incoming RTCP receiver reports.
type Sender struct {
	ClockRate       int
	Period          time.Duration
	TimeNow         func() time.Time
	WritePacketRTCP func(rtcp.Packet)

	mutex sync.RWMutex

	// data from RTP packets
	firstRTPPacketSent bool
	lastRTP            uint32
	lastNTP            time.Time
	lastSystem         time.Time
	localSSRC          uint32
	lastSequenceNumber uint16
	sent               uint64
	reportedLost       uint64
	octetCount         uint32

	terminate   chan struct{}
	done        chan struct{}
	firstPacket chan struct{}
}

// Initialize initializes a Sender.
func (rs *Sender) Initialize() {
	if rs.TimeNow == nil {
		rs.TimeNow = time.Now
	}

	rs.terminate = make(chan struct{})
	rs.done = make(chan struct{})
	rs.firstPacket = make(chan struct{})

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

	// RFC 6051, section 3.2: a sender SHOULD transmit an initial compound RTCP packet
	// immediately on joining a unicast session, so a receiver can establish the
	// RTP-to-wall-clock mapping without waiting for the first periodic report.
	//
	// Emitting only on the ticker delays that mapping by Period (10s by default), and
	// longer still when no RTP packet has been sent by the first tick -- report()
	// returns nil then, so the next opportunity is another Period away. Any receiver
	// that derives absolute time from sender reports is blind for that whole window on
	// every connection and every reconnect: mediamtx's useAbsoluteTimestamp drops the
	// packets outright, and GStreamer's rtpjitterbuffer attaches no
	// reference-timestamp-meta. See bluenviron/gortsplib#1052.
	//
	// The channel is read into a local: setting it to nil after it fires makes that
	// select case block forever, so the initial report is sent exactly once, without
	// racing ProcessPacket for ownership of the field.
	firstPacket := rs.firstPacket

	for {
		select {
		case <-firstPacket:
			firstPacket = nil

			report := rs.report()
			if report != nil {
				rs.WritePacketRTCP(report)
			}

			// Restart the period from the initial report, so a tick that was already
			// nearly due doesn't send a second report immediately after it.
			t.Reset(rs.Period)

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

	systemDiff := rs.TimeNow().Sub(rs.lastSystem)
	ntpTime := rs.lastNTP.Add(systemDiff)
	rtpTime := rs.lastRTP + uint32(systemDiff.Seconds()*float64(rs.ClockRate))

	return &rtcp.SenderReport{
		SSRC:        rs.localSSRC,
		NTPTime:     ntp.Encode(ntpTime),
		RTPTime:     rtpTime,
		PacketCount: uint32(rs.sent),
		OctetCount:  rs.octetCount,
	}
}

// ProcessPacket extracts data from RTP packets.
func (rs *Sender) ProcessPacket(pkt *rtp.Packet, ntp time.Time, ptsEqualsDTS bool) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	if ptsEqualsDTS {
		isFirst := !rs.firstRTPPacketSent

		rs.firstRTPPacketSent = true
		rs.lastRTP = pkt.Timestamp
		rs.lastNTP = ntp
		rs.lastSystem = rs.TimeNow()
		rs.localSSRC = pkt.SSRC

		// Wakes run() to emit the initial sender report now that report() has the data
		// it needs. Closed under the mutex and only on the false->true transition, so
		// it happens exactly once.
		if isFirst {
			close(rs.firstPacket)
		}
	}

	rs.lastSequenceNumber = pkt.SequenceNumber

	rs.sent++
	rs.octetCount += uint32(len(pkt.Payload))
}

// ProcessReceptionReport extracts data from RTCP receiver reports.
func (rs *Sender) ProcessReceptionReport(report *rtcp.ReceptionReport) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()

	rs.reportedLost = uint64(report.TotalLost)
}

// Stats are statistics.
type Stats struct {
	// outbound RTP packets.
	Sent uint64
	// last sequence number of outbound RTP packets.
	LastSequenceNumber uint16
	// last RTP time of outbound RTP packets.
	LastRTP uint32
	// last NTP time of outbound RTP packets.
	LastNTP time.Time
	// outbound RTP packets reported as lost by the remote receiver.
	ReportedLost uint64

	// Deprecated: use Sent.
	TotalSent uint64
}

// Stats returns statistics.
func (rs *Sender) Stats() *Stats {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()

	if !rs.firstRTPPacketSent {
		return nil
	}

	return &Stats{
		Sent:               rs.sent,
		LastSequenceNumber: rs.lastSequenceNumber,
		LastRTP:            rs.lastRTP,
		LastNTP:            rs.lastNTP,
		ReportedLost:       rs.reportedLost,
		// deprecated
		TotalSent: rs.sent,
	}
}
