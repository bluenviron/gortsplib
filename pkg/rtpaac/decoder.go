package rtpaac

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/pion/rtp"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

type decoderState int

const (
	decoderStateInitial decoderState = iota
	decoderStateReadingFragmented
)

// Decoder is a RTP/AAC decoder.
type Decoder struct {
	clockRate    time.Duration
	initialTs    uint32
	initialTsSet bool

	// for Decode()
	state         decoderState
	fragmentedBuf []byte
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

// Decode decodes AUs from a RTP/AAC packet.
// It returns the AUs and the PTS of the first AU.
// The PTS of following AUs can be calculated by adding time.Second*1000/clockRate.
func (d *Decoder) Decode(byts []byte) ([][]byte, time.Duration, error) {
	switch d.state {
	case decoderStateInitial:
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(byts)
		if err != nil {
			return nil, 0, err
		}

		if !d.initialTsSet {
			d.initialTsSet = true
			d.initialTs = pkt.Timestamp
		}

		if pkt.Header.Marker {
			if len(pkt.Payload) < 2 {
				return nil, 0, fmt.Errorf("payload is too short")
			}

			// AU-headers-length
			headersLen := binary.BigEndian.Uint16(pkt.Payload)
			if (headersLen % 16) != 0 {
				return nil, 0, fmt.Errorf("invalid AU-headers-length (%d)", headersLen)
			}
			pkt.Payload = pkt.Payload[2:]

			// AU-headers
			// AAC headers are 16 bits, where
			// * 13 bits are data size
			// * 3 bits are AU index
			headerCount := headersLen / 16
			var dataLens []uint16
			for i := 0; i < int(headerCount); i++ {
				if len(pkt.Payload[i*2:]) < 2 {
					return nil, 0, fmt.Errorf("payload is too short")
				}

				header := binary.BigEndian.Uint16(pkt.Payload[i*2:])
				dataLen := header >> 3
				auIndex := header & 0x03
				if auIndex != 0 {
					return nil, 0, fmt.Errorf("AU-index field is not zero")
				}

				dataLens = append(dataLens, dataLen)
			}
			pkt.Payload = pkt.Payload[headerCount*2:]

			// AUs
			aus := make([][]byte, len(dataLens))
			for i, dataLen := range dataLens {
				if len(pkt.Payload) < int(dataLen) {
					return nil, 0, fmt.Errorf("payload is too short")
				}

				aus[i] = pkt.Payload[:dataLen]
				pkt.Payload = pkt.Payload[dataLen:]
			}

			return aus, d.decodeTimestamp(pkt.Timestamp), nil
		}

		// AU-headers-length
		headersLen := binary.BigEndian.Uint16(pkt.Payload)
		if headersLen != 16 {
			return nil, 0, fmt.Errorf("invalid AU-headers-length (%d)", headersLen)
		}

		// AU-header
		header := binary.BigEndian.Uint16(pkt.Payload[2:])
		dataLen := header >> 3
		auIndex := header & 0x03
		if auIndex != 0 {
			return nil, 0, fmt.Errorf("AU-index field is not zero")
		}

		if len(pkt.Payload) < int(dataLen) {
			return nil, 0, fmt.Errorf("payload is too short")
		}

		d.fragmentedBuf = append(d.fragmentedBuf, pkt.Payload[4:]...)

		d.state = decoderStateReadingFragmented
		return nil, 0, ErrMorePacketsNeeded

	default: // decoderStateReadingFragmented
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(byts)
		if err != nil {
			d.state = decoderStateInitial
			return nil, 0, err
		}

		// AU-headers-length
		headersLen := binary.BigEndian.Uint16(pkt.Payload)
		if headersLen != 16 {
			return nil, 0, fmt.Errorf("invalid AU-headers-length (%d)", headersLen)
		}

		// AU-header
		header := binary.BigEndian.Uint16(pkt.Payload[2:])
		dataLen := header >> 3
		auIndex := header & 0x03
		if auIndex != 0 {
			return nil, 0, fmt.Errorf("AU-index field is not zero")
		}

		if len(pkt.Payload) < int(dataLen) {
			return nil, 0, fmt.Errorf("payload is too short")
		}

		d.fragmentedBuf = append(d.fragmentedBuf, pkt.Payload[4:]...)

		if !pkt.Header.Marker {
			return nil, 0, ErrMorePacketsNeeded
		}

		d.state = decoderStateInitial
		return [][]byte{d.fragmentedBuf}, d.decodeTimestamp(pkt.Timestamp), nil
	}
}
