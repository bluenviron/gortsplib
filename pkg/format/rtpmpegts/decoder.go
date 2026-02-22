package rtpmpegts

import (
	"fmt"

	"github.com/pion/rtp"
)

const (
	MPEGTSPacketSize = 188
	SyncByte         = 0x47
)

// Decoder is a RTP decoder for MPEG-TS codec that fit in a single packet.
type Decoder struct{}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	return nil
}

// Decode decodes MPEG-TS packets from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, error) {
	if len(pkt.Payload) == 0 {
		return nil, fmt.Errorf("empty MPEG-TS payload")
	}

	packetLen := len(pkt.Payload)
	if (packetLen % MPEGTSPacketSize) != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of %d", packetLen, MPEGTSPacketSize)
	}

	// validate sync byte at each 188-byte boundary
	for i := 0; i < packetLen; i += MPEGTSPacketSize {
		if pkt.Payload[i] != SyncByte {
			return nil, fmt.Errorf("missing sync byte at offset %d: got 0x%02x", i, pkt.Payload[i])
		}
	}

	return pkt.Payload, nil
}
