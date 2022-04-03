package rtpaac

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/aler9/gortsplib/pkg/rtptimedec"
	"github.com/icza/bitio"
	"github.com/pion/rtp"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

// Decoder is a RTP/AAC decoder.
type Decoder struct {
	// sample rate of input packets.
	SampleRate int

	timeDecoder          *rtptimedec.Decoder
	isDecodingFragmented bool
	fragmentedBuf        []byte

	// The number of bits on which the AU-size field is encoded in the AU-header.
	SizeLength int
	// The number of bits on which the AU-Index is encoded in the first AU-header.
	IndexLength int
}

// Init initializes the decoder
func (d *Decoder) Init() {
	d.timeDecoder = rtptimedec.New(d.SampleRate)
}

// Decode decodes AUs from a RTP/AAC packet.
// It returns the AUs and the PTS of the first AU.
// The PTS of subsequent AUs can be calculated by adding time.Second*1000/clockRate.
func (d *Decoder) Decode(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	if len(pkt.Payload) < 2 {
		d.isDecodingFragmented = false
		return nil, 0, fmt.Errorf("payload is too short")
	}

	// AU-headers-length
	headersLen := binary.BigEndian.Uint16(pkt.Payload)
	auHeaderSize := uint16(d.SizeLength + d.IndexLength)
	if auHeaderSize <= 0 {
		d.isDecodingFragmented = false
		return nil, 0, fmt.Errorf("invalid AU-header-size (%d)", auHeaderSize)
	}

	if (headersLen % auHeaderSize) != 0 {
		d.isDecodingFragmented = false
		return nil, 0, fmt.Errorf("invalid AU-headers-length (%d) with AU-header-size (%d)", headersLen, auHeaderSize)
	}
	headersLen_bytes := (int(headersLen) + 7) / 8
	payload := pkt.Payload[2:]

	if !d.isDecodingFragmented {
		if pkt.Header.Marker {
			// AU-headers
			// AAC headers are 16 bits, where
			// * 13 bits are data size
			// * 3 bits are AU index
			headerCount := headersLen / auHeaderSize
			dataLens, err := d.parseAuData(payload, headersLen_bytes, headerCount)
			if err != nil {
				return nil, 0, err
			}
			payload = payload[headersLen_bytes:]

			// AUs
			aus := make([][]byte, len(dataLens))
			for i, dataLen := range dataLens {
				if len(payload) < int(dataLen) {
					return nil, 0, fmt.Errorf("payload is too short")
				}

				aus[i] = payload[:dataLen]
				payload = payload[dataLen:]
			}

			return aus, d.timeDecoder.Decode(pkt.Timestamp), nil
		}

		if headersLen != auHeaderSize {
			return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
		}

		// AU-header
		dataLens, err := d.parseAuData(payload, headersLen_bytes, 1)
		if err != nil {
			return nil, 0, err
		}
		if len(dataLens) != 1 {
			return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
		}

		payload = payload[headersLen_bytes:]

		if len(payload) < int(dataLens[0]) {
			return nil, 0, fmt.Errorf("payload is too short")
		}

		d.fragmentedBuf = append(d.fragmentedBuf, payload...)

		d.isDecodingFragmented = true
		return nil, 0, ErrMorePacketsNeeded
	}

	// we are decoding a fragmented AU

	if headersLen != auHeaderSize {
		return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
	}

	// AU-header
	dataLens, err := d.parseAuData(payload, headersLen_bytes, 1)
	if err != nil {
		return nil, 0, err
	}
	if len(dataLens) != 1 {
		return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
	}
	payload = payload[headersLen_bytes:]

	if len(payload) < int(dataLens[0]) {
		return nil, 0, fmt.Errorf("payload is too short")
	}

	d.fragmentedBuf = append(d.fragmentedBuf, payload...)

	if !pkt.Header.Marker {
		return nil, 0, ErrMorePacketsNeeded
	}

	d.isDecodingFragmented = false
	return [][]byte{d.fragmentedBuf}, d.timeDecoder.Decode(pkt.Timestamp), nil
}

func (d *Decoder) parseAuData(payload []byte,
	headersLen_bytes int,
	headerCount uint16,
) (dataLens []uint64, err error) {
	if len(payload) < headersLen_bytes {
		return nil, fmt.Errorf("payload is too short")
	}

	br := bitio.NewReader(bytes.NewBuffer(payload[:headersLen_bytes]))
	for i := 0; i < int(headerCount); i++ {
		dataLen, err := br.ReadBits(uint8(d.SizeLength))
		if err != nil {
			return nil, fmt.Errorf("payload is too short")
		}
		if d.IndexLength > 0 {
			auIndex, err := br.ReadBits(uint8(d.IndexLength))
			if err != nil {
				return nil, fmt.Errorf("payload is too short")
			}

			if auIndex != 0 {
				return nil, fmt.Errorf("AU-index field is not zero")
			}
		}
		dataLens = append(dataLens, dataLen)
	}
	return dataLens, nil
}
