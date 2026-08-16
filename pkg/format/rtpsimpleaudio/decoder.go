package rtpsimpleaudio

import (
	"fmt"

	"github.com/pion/rtp"
)

// Decoder is a RTP decoder for audio codecs that fit in a single packet.
type Decoder struct{}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	return nil
}

// Decode decodes an audio frame from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, error) {
	if len(pkt.Payload) == 0 {
		return nil, fmt.Errorf("payload is too short")
	}

	return pkt.Payload, nil
}
