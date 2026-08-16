package rtplpcm

import (
	"fmt"

	"github.com/pion/rtp"
)

// Decoder is a RTP/LPCM decoder.
// Specification: RFC3190
type Decoder struct {
	// Deprecated: not needed anymore.
	BitDepth int

	// Deprecated: not needed anymore.
	ChannelCount int
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	return nil
}

// Decode decodes audio samples from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, error) {
	if len(pkt.Payload) == 0 {
		return nil, fmt.Errorf("payload is too short")
	}

	return pkt.Payload, nil
}
