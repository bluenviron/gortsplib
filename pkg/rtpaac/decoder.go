package rtpaac

import (
	"encoding/binary"
	"fmt"
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

// Decode decodes one or multiple AUs from an RTP/AAC packet.
func (d *Decoder) Decode(byts []byte) ([]*AUAndTimestamp, error) {
	pkt := rtp.Packet{}
	err := pkt.Unmarshal(byts)
	if err != nil {
		return nil, err
	}

	if !d.initialTsSet {
		d.initialTsSet = true
		d.initialTs = pkt.Timestamp
	}

	// AU-headers-length
	headersLen := binary.BigEndian.Uint16(pkt.Payload)
	if (headersLen % 16) != 0 {
		return nil, fmt.Errorf("invalid AU-headers-length (%d)", headersLen)
	}
	pkt.Payload = pkt.Payload[2:]

	// AU-headers
	// AAC headers are 16 bits, where
	// * 13 bits are data size
	// * 3 bits are AU index
	headerCount := headersLen / 16
	var dataSizes []uint16
	for i := 0; i < int(headerCount); i++ {
		header := binary.BigEndian.Uint16(pkt.Payload[i*2:])
		dataSize := header >> 3
		auIndex := header & 0x03
		if auIndex != 0 {
			return nil, fmt.Errorf("AU-index field must be zero")
		}

		dataSizes = append(dataSizes, dataSize)
	}
	pkt.Payload = pkt.Payload[headerCount*2:]

	ts := d.decodeTimestamp(pkt.Timestamp)
	rets := make([]*AUAndTimestamp, len(dataSizes))

	for i, ds := range dataSizes {
		if len(pkt.Payload) < int(ds) {
			return nil, fmt.Errorf("payload is too short")
		}

		rets[i] = &AUAndTimestamp{
			AU:        pkt.Payload[:ds],
			Timestamp: ts + time.Duration(i)*1000*time.Second/d.clockRate,
		}

		pkt.Payload = pkt.Payload[ds:]
	}

	return rets, nil
}
