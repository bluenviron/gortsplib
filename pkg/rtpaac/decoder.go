package rtpaac

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/icza/bitio"
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/rtptimedec"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

// Decoder is a RTP/AAC decoder.
type Decoder struct {
	// sample rate of input packets.
	SampleRate int

	timeDecoder     *rtptimedec.Decoder
	fragmentedMode  bool
	fragmentedParts [][]byte
	fragmentedSize  int

	// The number of bits on which the AU-size field is encoded in the AU-header.
	SizeLength int
	// The number of bits on which the AU-Index is encoded in the first AU-header.
	IndexLength int
	// The number of bits on which the AU-Index-delta field is encoded in any non-first AU-header.
	IndexDeltaLength int
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
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("payload is too short")
	}

	// AU-headers-length
	headersLen := int(binary.BigEndian.Uint16(pkt.Payload))

	auHeaderSize := d.SizeLength + d.IndexLength
	if auHeaderSize <= 0 {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("invalid AU-header-size (%d)", auHeaderSize)
	}

	if (headersLen % auHeaderSize) != 0 {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("invalid AU-headers-length (%d) with AU-header-size (%d)", headersLen, auHeaderSize)
	}
	headersLenBytes := (headersLen + 7) / 8
	payload := pkt.Payload[2:]

	if !d.fragmentedMode {
		if pkt.Header.Marker {
			// AU-headers
			// AAC headers are 16 bits, where
			// * 13 bits are data size
			// * 3 bits are AU index
			headerCount := headersLen / auHeaderSize
			dataLens, err := d.parseAuData(payload, headersLenBytes, headerCount)
			if err != nil {
				return nil, 0, err
			}
			payload = payload[headersLenBytes:]

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
		dataLens, err := d.parseAuData(payload, headersLenBytes, 1)
		if err != nil {
			return nil, 0, err
		}
		if len(dataLens) != 1 {
			return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
		}
		payload = payload[headersLenBytes:]

		if len(payload) < int(dataLens[0]) {
			return nil, 0, fmt.Errorf("payload is too short")
		}

		d.fragmentedParts = append(d.fragmentedParts, payload)
		d.fragmentedSize = len(payload)
		d.fragmentedMode = true
		return nil, 0, ErrMorePacketsNeeded
	}

	// we are decoding a fragmented AU

	if headersLen != auHeaderSize {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
	}

	// AU-header
	dataLens, err := d.parseAuData(payload, headersLenBytes, 1)
	if err != nil {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, err
	}
	if len(dataLens) != 1 {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
	}
	payload = payload[headersLenBytes:]

	if len(payload) < int(dataLens[0]) {
		return nil, 0, fmt.Errorf("payload is too short")
	}

	d.fragmentedSize += len(payload)
	if d.fragmentedSize > maxAUSize {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("AU size (%d) is too big (maximum is %d)", d.fragmentedSize, maxAUSize)
	}

	d.fragmentedParts = append(d.fragmentedParts, payload)

	if !pkt.Header.Marker {
		return nil, 0, ErrMorePacketsNeeded
	}

	ret := make([]byte, d.fragmentedSize)
	n := 0
	for _, p := range d.fragmentedParts {
		n += copy(ret[n:], p)
	}

	d.fragmentedParts = d.fragmentedParts[:0]
	d.fragmentedMode = false
	return [][]byte{ret}, d.timeDecoder.Decode(pkt.Timestamp), nil
}

func (d *Decoder) parseAuData(payload []byte,
	headersLenBytes int,
	headerCount int,
) (dataLens []uint64, err error) {
	if len(payload) < headersLenBytes {
		return nil, fmt.Errorf("payload is too short")
	}

	br := bitio.NewReader(bytes.NewBuffer(payload[:headersLenBytes]))
	readAUIndex := func(index int) error {
		auIndex, err := br.ReadBits(uint8(index))
		if err != nil {
			return fmt.Errorf("payload is too short")
		}

		if auIndex != 0 {
			return fmt.Errorf("AU-index field is not zero")
		}

		return nil
	}
	for i := 0; i < headerCount; i++ {
		dataLen, err := br.ReadBits(uint8(d.SizeLength))
		if err != nil {
			return nil, fmt.Errorf("payload is too short")
		}
		switch {
		case i == 0 && d.IndexLength > 0:
			err := readAUIndex(d.IndexLength)
			if err != nil {
				return nil, err
			}
		case d.IndexDeltaLength > 0:
			err := readAUIndex(d.IndexDeltaLength)
			if err != nil {
				return nil, err
			}
		}

		dataLens = append(dataLens, dataLen)
	}
	return dataLens, nil
}
