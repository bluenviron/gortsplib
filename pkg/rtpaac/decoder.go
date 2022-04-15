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

	// The number of bits on which the AU-size field is encoded in the AU-header (optional).
	// It defaults to 13.
	SizeLength *int

	// The number of bits on which the AU-Index is encoded in the first AU-header (optional).
	// It defaults to 3.
	IndexLength *int

	// The number of bits on which the AU-Index-delta field is encoded in any non-first AU-header (optional).
	// It defaults to 3.
	IndexDeltaLength *int

	timeDecoder     *rtptimedec.Decoder
	fragmentedMode  bool
	fragmentedParts [][]byte
	fragmentedSize  int
}

// Init initializes the decoder
func (d *Decoder) Init() {
	if d.SizeLength == nil {
		v := 13
		d.SizeLength = &v
	}
	if d.IndexLength == nil {
		v := 3
		d.IndexLength = &v
	}
	if d.IndexDeltaLength == nil {
		v := 3
		d.IndexDeltaLength = &v
	}

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

	// AU-headers-length (16 bits)
	headersLen := int(binary.BigEndian.Uint16(pkt.Payload))
	if headersLen == 0 || (headersLen%8) != 0 {
		return nil, 0, fmt.Errorf("invalid AU-headers-length (%d)", headersLen)
	}
	payload := pkt.Payload[2:]

	// AU-headers
	dataLens, err := d.readAUHeaders(payload, headersLen)
	if err != nil {
		return nil, 0, err
	}
	payload = payload[(headersLen / 8):]

	if !d.fragmentedMode {
		if pkt.Header.Marker {
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

		if len(dataLens) != 1 {
			return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
		}

		if len(payload) < int(dataLens[0]) {
			return nil, 0, fmt.Errorf("payload is too short")
		}

		d.fragmentedSize = int(dataLens[0])
		d.fragmentedParts = append(d.fragmentedParts, payload[:dataLens[0]])
		d.fragmentedMode = true
		return nil, 0, ErrMorePacketsNeeded
	}

	// we are decoding a fragmented AU

	if len(dataLens) != 1 {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
	}

	if len(payload) < int(dataLens[0]) {
		return nil, 0, fmt.Errorf("payload is too short")
	}

	d.fragmentedSize += int(dataLens[0])
	if d.fragmentedSize > maxAUSize {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("AU size (%d) is too big (maximum is %d)", d.fragmentedSize, maxAUSize)
	}

	d.fragmentedParts = append(d.fragmentedParts, payload[:dataLens[0]])

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

func (d *Decoder) readAUHeaders(payload []byte, headersLen int) ([]uint64, error) {
	br := bitio.NewReader(bytes.NewBuffer(payload))
	firstRead := false

	count := 0
	for i := 0; i < headersLen; {
		if i == 0 {
			i += *d.SizeLength
			i += *d.IndexLength
		} else {
			i += *d.SizeLength
			i += *d.IndexDeltaLength
		}
		count++
	}

	dataLens := make([]uint64, count)

	i := 0
	for headersLen > 0 {
		dataLen, err := br.ReadBits(uint8(*d.SizeLength))
		if err != nil {
			return nil, err
		}
		headersLen -= *d.SizeLength

		if !firstRead {
			firstRead = true
			if *d.IndexLength > 0 {
				auIndex, err := br.ReadBits(uint8(*d.IndexLength))
				if err != nil {
					return nil, err
				}
				headersLen -= *d.IndexLength

				if auIndex != 0 {
					return nil, fmt.Errorf("AU-index different than zero is not supported")
				}
			}
		} else if *d.IndexDeltaLength > 0 {
			auIndexDelta, err := br.ReadBits(uint8(*d.IndexDeltaLength))
			if err != nil {
				return nil, err
			}
			headersLen -= *d.IndexDeltaLength

			if auIndexDelta != 0 {
				return nil, fmt.Errorf("AU-index-delta different than zero is not supported")
			}
		}

		dataLens[i] = dataLen
		i++
	}

	return dataLens, nil
}
