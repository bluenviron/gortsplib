package rtpmpegts

import (
	"fmt"

	"github.com/pion/rtp"
)

const (
	mpegtsPacketSize = 188
	syncByte         = 0x47
)

// Decoder is a RTP/MPEG-TS decoder.
// Specification: RFC2250
type Decoder struct{}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	return nil
}

// Decode decodes MPEG-TS packets from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([][]byte, error) {
	if len(pkt.Payload) == 0 {
		return nil, fmt.Errorf("empty MPEG-TS payload")
	}

	packetLen := len(pkt.Payload)
	if (packetLen % mpegtsPacketSize) != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of %d", packetLen, mpegtsPacketSize)
	}

	tsPacketCount := packetLen / mpegtsPacketSize
	ret := make([][]byte, tsPacketCount)

	// validate sync byte at each 188-byte boundary
	for i := range ret {
		j := i * mpegtsPacketSize

		if pkt.Payload[j] != syncByte {
			return nil, fmt.Errorf("missing sync byte at offset %d: got 0x%02x", j, pkt.Payload[j])
		}

		ret[i] = pkt.Payload[j : j+mpegtsPacketSize]
	}

	return ret, nil
}
