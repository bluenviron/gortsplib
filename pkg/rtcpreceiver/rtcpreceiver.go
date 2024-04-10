// Package rtcpreceiver contains a utility to generate RTCP receiver reports.
package rtcpreceiver

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

// seconds since 1st January 1900
// higher 32 bits are the integer part, lower 32 bits are the fractional part
func ntpTimeRTCPToGo(v uint64) time.Time {
	nano := int64((v>>32)*1000000000+(v&0xFFFFFFFF)) - 2208988800*1000000000
	return time.Unix(0, nano)
}

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

// RTCPReceiver is a utility to generate RTCP receiver reports.
type RTCPReceiver struct {
	clockRate       float64
	receiverSSRC    uint32
	period          time.Duration
	timeNow         func() time.Time
	writePacketRTCP func(rtcp.Packet)
	mutex           sync.RWMutex

	// data from RTP packets
	firstRTPPacketReceived bool
	timeInitialized        bool
	sequenceNumberCycles   uint16
	lastSequenceNumber     uint16
	senderSSRC             uint32
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

// New allocates a RTCPReceiver.
func New(
	clockRate int,
	receiverSSRC *uint32,
	period time.Duration,
	timeNow func() time.Time,
	writePacketRTCP func(rtcp.Packet),
) (*RTCPReceiver, error) {
	if receiverSSRC == nil {
		v, err := randUint32()
		if err != nil {
			return nil, err
		}
		receiverSSRC = &v
	}

	if timeNow == nil {
		timeNow = time.Now
	}

	rr := &RTCPReceiver{
		clockRate:       float64(clockRate),
		receiverSSRC:    *receiverSSRC,
		period:          period,
		timeNow:         timeNow,
		writePacketRTCP: writePacketRTCP,
		terminate:       make(chan struct{}),
		done:            make(chan struct{}),
	}

	go rr.run()

	return rr, nil
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
			report := rr.report()
			if report != nil {
				rr.writePacketRTCP(report)
			}

		case <-rr.terminate:
			return
		}
	}
}

func (rr *RTCPReceiver) report() rtcp.Packet {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	if !rr.firstRTPPacketReceived {
		return nil
	}

	system := rr.timeNow()

	report := &rtcp.ReceiverReport{
		SSRC: rr.receiverSSRC,
		Reports: []rtcp.ReceptionReport{
			{
				SSRC:               rr.senderSSRC,
				LastSequenceNumber: uint32(rr.sequenceNumberCycles)<<16 | uint32(rr.lastSequenceNumber),
				// equivalent to taking the integer part after multiplying the
				// loss fraction by 256
				FractionLost: uint8(float64(rr.totalLostSinceReport*256) / float64(rr.totalSinceReport)),
				TotalLost:    rr.totalLost,
				Jitter:       uint32(rr.jitter),
			},
		},
	}

	if rr.firstSenderReportReceived {
		// middle 32 bits out of 64 in the NTP timestamp of last sender report
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

// ProcessPacket extracts the needed data from RTP packets.
func (rr *RTCPReceiver) ProcessPacket(pkt *rtp.Packet, system time.Time, ptsEqualsDTS bool) error {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	// first packet
	if !rr.firstRTPPacketReceived {
		rr.firstRTPPacketReceived = true
		rr.totalSinceReport = 1
		rr.lastSequenceNumber = pkt.SequenceNumber
		rr.senderSSRC = pkt.SSRC

		if ptsEqualsDTS {
			rr.timeInitialized = true
			rr.lastTimeRTP = pkt.Timestamp
			rr.lastTimeSystem = system
		}

		// subsequent packets
	} else {
		if pkt.SSRC != rr.senderSSRC {
			return fmt.Errorf("received packet with wrong SSRC %d, expected %d", pkt.SSRC, rr.senderSSRC)
		}

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
		rr.lastSequenceNumber = pkt.SequenceNumber

		if ptsEqualsDTS {
			if rr.timeInitialized {
				// update jitter
				// https://tools.ietf.org/html/rfc3550#page-39
				D := system.Sub(rr.lastTimeSystem).Seconds()*rr.clockRate -
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

	return nil
}

// ProcessSenderReport extracts the needed data from RTCP sender reports.
func (rr *RTCPReceiver) ProcessSenderReport(sr *rtcp.SenderReport, system time.Time) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	rr.firstSenderReportReceived = true
	rr.lastSenderReportTimeNTP = sr.NTPTime
	rr.lastSenderReportTimeRTP = sr.RTPTime
	rr.lastSenderReportTimeSystem = system
}

// PacketNTP returns the NTP timestamp of the packet.
func (rr *RTCPReceiver) PacketNTP(ts uint32) (time.Time, bool) {
	rr.mutex.Lock()
	defer rr.mutex.Unlock()

	if !rr.firstSenderReportReceived {
		return time.Time{}, false
	}

	timeDiff := int32(ts - rr.lastSenderReportTimeRTP)
	timeDiffGo := (time.Duration(timeDiff) * time.Second) / time.Duration(rr.clockRate)

	return ntpTimeRTCPToGo(rr.lastSenderReportTimeNTP).Add(timeDiffGo), true
}

// SenderSSRC returns the SSRC of outgoing RTP packets.
func (rr *RTCPReceiver) SenderSSRC() (uint32, bool) {
	rr.mutex.RLock()
	defer rr.mutex.RUnlock()
	return rr.senderSSRC, rr.firstRTPPacketReceived
}
