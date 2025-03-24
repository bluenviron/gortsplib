// Package rtplossdetector implements an algorithm that detects lost packets.
package rtplossdetector

import (
	"github.com/bluenviron/gortsplib/v4/internal/rtplossdetector"
	"github.com/pion/rtp"
)

// LossDetector detects lost packets.
//
// Deprecated: will be removed in the next version.
type LossDetector rtplossdetector.LossDetector

// New allocates a LossDetector.
func New() *LossDetector {
	return &LossDetector{}
}

// Process processes a RTP packet.
// It returns the number of lost packets.
func (r *LossDetector) Process(pkt *rtp.Packet) uint {
	return uint((*rtplossdetector.LossDetector)(r).Process(pkt))
}
