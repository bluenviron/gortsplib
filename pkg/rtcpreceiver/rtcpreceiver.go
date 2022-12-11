// Package rtcpreceiver contains a utility to generate RTCP receiver reports.
package rtcpreceiver

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

func randUint32() uint32 {
	var b [4]byte
	rand.Read(b[:])
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

var now = time.Now

// RTCPReceiver is a utility to generate RTCP receiver reports.
type RTCPReceiver struct {
	period          time.Duration
	receiverSSRC    uint32
	clockRate       float64
	writePacketRTCP func(rtcp.Packet)
	mutex           sync.Mutex

	// data from RTP packets
	initialized          bool
	timeInitialized      bool
	sequenceNumberCycles uint16
	lastSSRC             uint32
	lastSequenceNumber   uint16
	lastTimeRTP          uint32
	lastTimeNTP          time.Time
	totalLost            uint32
	totalLostSinceReport uint32
	totalSinceReport     uint32
	jitter               float64

	// data from RTCP packets
	senderInitialized    bool
	lastSenderReportNTP  uint32
	lastSenderReportTime time.Time

	terminate chan struct{}
	done      chan struct{}
}

// New allocates a RTCPReceiver.
func New(
	period time.Duration,
	receiverSSRC *uint32,
	clockRate int,
	writePacketRTCP func(rtcp.Packet),
) *RTCPReceiver {
	rr := &RTCPReceiver{
		period: period,
		receiverSSRC: func() uint32 {
			if receiverSSRC == nil {
				return randUint32()
			}
			return *receiverSSRC
		}(),
		clockRate:       float64(clockRate),
		writePacketRTCP: writePacketRTCP,
		terminate:       make(chan struct{}),
		done:            make(chan struct{}),
	}

	go rr.run()

	return rr
}

// Close closes the RTCPReceiver.
func (rr *RTCPReceiver) Close() {
	close(rr.terminate)
	<-rr.done
}

func (rr *RTCPReceiver) run() {
	defer close(rr.done)

	t := time.NewTicker(rr.period)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			report := rr.report(now())
			if report != nil {
				rr.writePacketRTCP(report)
			}

		case <-rr.terminate:
			return
		}
	}
}

func (rr *RTCPReceiver) report(ts time.Time) rtcp.Packet {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	if !rr.senderInitialized || !rr.initialized || rr.clockRate == 0 {
		return nil
	}

	report := &rtcp.ReceiverReport{
		SSRC: rr.receiverSSRC,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               rr.lastSSRC,
				LastSequenceNumber: uint32(rr.sequenceNumberCycles)<<16 | uint32(rr.lastSequenceNumber),
				// middle 32 bits out of 64 in the NTP timestamp of last sender report
				LastSenderReport: rr.lastSenderReportNTP,
				// equivalent to taking the integer part after multiplying the
				// loss fraction by 256
				FractionLost: uint8(float64(rr.totalLostSinceReport*256) / float64(rr.totalSinceReport)),
				TotalLost:    rr.totalLost,
				// delay, expressed in units of 1/65536 seconds, between
				// receiving the last SR packet from source SSRC_n and sending this
				// reception report block
				Delay:  uint32(ts.Sub(rr.lastSenderReportTime).Seconds() * 65536),
				Jitter: uint32(rr.jitter),
			},
		},
	}

	rr.totalLostSinceReport = 0
	rr.totalSinceReport = 0

	return report
}

// ProcessPacket extracts the needed data from RTP packets.
func (rr *RTCPReceiver) ProcessPacket(pkt *rtp.Packet, ntp time.Time, ptsEqualsDTS bool) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	// first packet
	if !rr.initialized {
		rr.initialized = true
		rr.totalSinceReport = 1
		rr.lastSSRC = pkt.SSRC
		rr.lastSequenceNumber = pkt.SequenceNumber

		if ptsEqualsDTS {
			rr.timeInitialized = true
			rr.lastTimeRTP = pkt.Timestamp
			rr.lastTimeNTP = ntp
		}

		// subsequent packets
	} else {
		diff := int32(pkt.SequenceNumber) - int32(rr.lastSequenceNumber)

		// overflow
		if diff < -0x0FFF {
			rr.sequenceNumberCycles++
		}

		// detect lost packets
		if pkt.SequenceNumber != (rr.lastSequenceNumber + 1) {
			rr.totalLost += uint32(uint16(diff) - 1)
			rr.totalLostSinceReport += uint32(uint16(diff) - 1)

			// allow up to 24 bits
			if rr.totalLost > 0xFFFFFF {
				rr.totalLost = 0xFFFFFF
			}
			if rr.totalLostSinceReport > 0xFFFFFF {
				rr.totalLostSinceReport = 0xFFFFFF
			}
		}

		rr.totalSinceReport += uint32(uint16(diff))
		rr.lastSSRC = pkt.SSRC
		rr.lastSequenceNumber = pkt.SequenceNumber

		if ptsEqualsDTS {
			if rr.timeInitialized {
				// update jitter
				// https://tools.ietf.org/html/rfc3550#page-39
				D := ntp.Sub(rr.lastTimeNTP).Seconds()*rr.clockRate -
					(float64(pkt.Timestamp) - float64(rr.lastTimeRTP))
				if D < 0 {
					D = -D
				}
				rr.jitter += (D - rr.jitter) / 16
			}

			rr.timeInitialized = true
			rr.lastTimeRTP = pkt.Timestamp
			rr.lastTimeNTP = ntp
			rr.lastSSRC = pkt.SSRC
		}
	}
}

// ProcessSenderReport extracts the needed data from RTCP sender reports.
func (rr *RTCPReceiver) ProcessSenderReport(sr *rtcp.SenderReport, ts time.Time) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	rr.senderInitialized = true
	rr.lastSenderReportNTP = uint32(sr.NTPTime >> 16)
	rr.lastSenderReportTime = ts
}

// LastSSRC returns the SSRC of the last RTP packet.
func (rr *RTCPReceiver) LastSSRC() (uint32, bool) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()
	return rr.lastSSRC, rr.initialized
}
