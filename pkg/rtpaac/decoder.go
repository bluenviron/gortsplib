package rtpaac

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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

	// for Read()
	readQueue []*AUAndTimestamp
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
func (d *Decoder) Decode(byts []byte) ([]*AUAndTimestamp, error) {
	switch d.state {
	case decoderStateInitial:
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(byts)
		if err != nil {
			return nil, err
		}

		if !d.initialTsSet {
			d.initialTsSet = true
			d.initialTs = pkt.Timestamp
		}

		if pkt.Header.Marker {
			if len(pkt.Payload) < 2 {
				return nil, fmt.Errorf("payload is too short")
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
			var dataLens []uint16
			for i := 0; i < int(headerCount); i++ {
				if len(pkt.Payload[i*2:]) < 2 {
					return nil, fmt.Errorf("payload is too short")
				}

				header := binary.BigEndian.Uint16(pkt.Payload[i*2:])
				dataLen := header >> 3
				auIndex := header & 0x03
				if auIndex != 0 {
					return nil, fmt.Errorf("AU-index field is not zero")
				}

				dataLens = append(dataLens, dataLen)
			}
			pkt.Payload = pkt.Payload[headerCount*2:]

			ts := d.decodeTimestamp(pkt.Timestamp)
			rets := make([]*AUAndTimestamp, len(dataLens))

			// AUs
			for i, dataLen := range dataLens {
				if len(pkt.Payload) < int(dataLen) {
					return nil, fmt.Errorf("payload is too short")
				}

				rets[i] = &AUAndTimestamp{
					AU:        pkt.Payload[:dataLen],
					Timestamp: ts + time.Duration(i)*1000*time.Second/d.clockRate,
				}

				pkt.Payload = pkt.Payload[dataLen:]
			}

			return rets, nil
		}

		// AU-headers-length
		headersLen := binary.BigEndian.Uint16(pkt.Payload)
		if headersLen != 16 {
			return nil, fmt.Errorf("invalid AU-headers-length (%d)", headersLen)
		}

		// AU-header
		header := binary.BigEndian.Uint16(pkt.Payload[2:])
		dataLen := header >> 3
		auIndex := header & 0x03
		if auIndex != 0 {
			return nil, fmt.Errorf("AU-index field is not zero")
		}

		if len(pkt.Payload) < int(dataLen) {
			return nil, fmt.Errorf("payload is too short")
		}

		d.fragmentedBuf = append(d.fragmentedBuf, pkt.Payload[4:]...)

		d.state = decoderStateReadingFragmented
		return nil, ErrMorePacketsNeeded

	default: // decoderStateReadingFragmented
		pkt := rtp.Packet{}
		err := pkt.Unmarshal(byts)
		if err != nil {
			d.state = decoderStateInitial
			return nil, err
		}

		// AU-headers-length
		headersLen := binary.BigEndian.Uint16(pkt.Payload)
		if headersLen != 16 {
			return nil, fmt.Errorf("invalid AU-headers-length (%d)", headersLen)
		}

		// AU-header
		header := binary.BigEndian.Uint16(pkt.Payload[2:])
		dataLen := header >> 3
		auIndex := header & 0x03
		if auIndex != 0 {
			return nil, fmt.Errorf("AU-index field is not zero")
		}

		if len(pkt.Payload) < int(dataLen) {
			return nil, fmt.Errorf("payload is too short")
		}

		d.fragmentedBuf = append(d.fragmentedBuf, pkt.Payload[4:]...)

		if !pkt.Header.Marker {
			return nil, ErrMorePacketsNeeded
		}

		d.state = decoderStateInitial
		return []*AUAndTimestamp{{
			AU:        d.fragmentedBuf,
			Timestamp: d.decodeTimestamp(pkt.Timestamp),
		}}, nil
	}
}

// Read reads RTP/AAC packets from a reader until an AU is decoded.
func (d *Decoder) Read(r io.Reader) (*AUAndTimestamp, error) {
	if len(d.readQueue) > 0 {
		au := d.readQueue[0]
		d.readQueue = d.readQueue[1:]
		return au, nil
	}

	buf := make([]byte, 2048)
	for {
		n, err := r.Read(buf)
		if err != nil {
			return nil, err
		}

		aus, err := d.Decode(buf[:n])
		if err != nil {
			if err == ErrMorePacketsNeeded {
				continue
			}
			return nil, err
		}

		au := aus[0]
		d.readQueue = aus[1:]

		return au, nil
	}
}
