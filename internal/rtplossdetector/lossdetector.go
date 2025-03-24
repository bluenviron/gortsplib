// Package rtplossdetector implements an algorithm that detects lost packets.
package rtplossdetector

import (
	"github.com/pion/rtp"
)

// LossDetector detects lost packets.
type LossDetector struct {
	initialized    bool
	expectedSeqNum uint16
}

// Process processes a RTP packet.
// It returns the number of lost packets.
func (r *LossDetector) Process(pkt *rtp.Packet) uint64 {
	if !r.initialized {
		r.initialized = true
		r.expectedSeqNum = pkt.SequenceNumber + 1
		return 0
	}

	if pkt.SequenceNumber != r.expectedSeqNum {
		diff := pkt.SequenceNumber - r.expectedSeqNum
		r.expectedSeqNum = pkt.SequenceNumber + 1
		return uint64(diff)
	}

	r.expectedSeqNum = pkt.SequenceNumber + 1
	return 0
}
