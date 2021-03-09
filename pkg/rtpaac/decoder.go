package rtpaac

import (
	"time"

	"github.com/pion/rtp"
)

// Decoder is a RTP/AAC decoder.
type Decoder struct {
	clockRate    time.Duration
	initialTs    uint32
	initialTsSet bool
}

// NewDecoder allocates a Decoder.
func NewDecoder(clockRate int) *Decoder {
	return &Decoder{
		clockRate: time.Duration(clockRate),
	}
}

func (d *Decoder) decodeTimestamp(ts uint32) time.Duration {
	return (time.Duration(ts) - time.Duration(d.initialTs)) * time.Second / d.clockRate
}

// Decode decodes an AU from an RTP/AAC packet.
func (d *Decoder) Decode(byts []byte) (*AUAndTimestamp, error) {
	pkt := rtp.Packet{}
	err := pkt.Unmarshal(byts)
	if err != nil {
		return nil, err
	}

	if !d.initialTsSet {
		d.initialTsSet = true
		d.initialTs = pkt.Timestamp
	}

	return &AUAndTimestamp{
		AU:        pkt.Payload[4:],
		Timestamp: d.decodeTimestamp(pkt.Timestamp),
	}, nil
}
