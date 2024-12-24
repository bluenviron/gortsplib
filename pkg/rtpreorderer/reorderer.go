// Package rtpreorderer implements a filter to reorder incoming RTP packets.
package rtpreorderer

import (
	"github.com/bluenviron/gortsplib/v4/internal/rtpreorderer"
	"github.com/pion/rtp"
)

// Reorderer filters incoming RTP packets, in order to
// - order packets
// - remove duplicate packets
//
// Deprecated: will be removed in the next version.
type Reorderer rtpreorderer.Reorderer

// New allocates a Reorderer.
func New() *Reorderer {
	r := &rtpreorderer.Reorderer{}
	r.Initialize()
	return (*Reorderer)(r)
}

// Process processes a RTP packet.
// It returns a sequence of ordered packets and the number of lost packets.
func (r *Reorderer) Process(pkt *rtp.Packet) ([]*rtp.Packet, int) {
	return (*rtpreorderer.Reorderer)(r).Process(pkt)
}
