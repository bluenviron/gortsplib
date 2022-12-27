package rtpmpeg4audio

import (
	"errors"
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/bits"
	"github.com/aler9/gortsplib/v2/pkg/codecs/mpeg4audio"
	"github.com/aler9/gortsplib/v2/pkg/rtptimedec"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

// Decoder is a RTP/MPEG4-audio decoder.
type Decoder struct {
	// sample rate of input packets.
	SampleRate int

	// The number of bits in which the AU-size field is encoded in the AU-header.
	SizeLength int

	// The number of bits in which the AU-Index is encoded in the first AU-header.
	IndexLength int

	// The number of bits in which the AU-Index-delta field is encoded in any non-first AU-header.
	IndexDeltaLength int

	timeDecoder    *rtptimedec.Decoder
	firstAUParsed  bool
	adtsMode       bool
	fragments      [][]byte
	fragmentedSize int
}

// Init initializes the decoder.
func (d *Decoder) Init() {
	d.timeDecoder = rtptimedec.New(d.SampleRate)
}

// Decode decodes AUs from a RTP/MPEG4-audio packet.
// It returns the AUs and the PTS of the first AU.
// The PTS of subsequent AUs can be calculated by adding time.Second*mpeg4audio.SamplesPerAccessUnit/clockRate.
func (d *Decoder) Decode(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	if len(pkt.Payload) < 2 {
		d.fragments = d.fragments[:0] // discard pending fragmented packets
		return nil, 0, fmt.Errorf("payload is too short")
	}

	// AU-headers-length (16 bits)
	headersLen := int(uint16(pkt.Payload[0])<<8 | uint16(pkt.Payload[1]))
	if headersLen == 0 {
		return nil, 0, fmt.Errorf("invalid AU-headers-length")
	}
	payload := pkt.Payload[2:]

	// AU-headers
	dataLens, err := d.readAUHeaders(payload, headersLen)
	if err != nil {
		d.fragments = d.fragments[:0] // discard pending fragmented packets
		return nil, 0, err
	}

	pos := (headersLen / 8)
	if (headersLen % 8) != 0 {
		pos++
	}
	payload = payload[pos:]

	var aus [][]byte

	if len(d.fragments) == 0 {
		if pkt.Header.Marker {
			// AUs
			aus = make([][]byte, len(dataLens))
			for i, dataLen := range dataLens {
				if len(payload) < int(dataLen) {
					return nil, 0, fmt.Errorf("payload is too short")
				}

				aus[i] = payload[:dataLen]
				payload = payload[dataLen:]
			}
		} else {
			if len(dataLens) != 1 {
				return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
			}

			if len(payload) < int(dataLens[0]) {
				return nil, 0, fmt.Errorf("payload is too short")
			}

			d.fragmentedSize = int(dataLens[0])
			d.fragments = append(d.fragments, payload[:dataLens[0]])
			return nil, 0, ErrMorePacketsNeeded
		}
	} else {
		// we are decoding a fragmented AU
		if len(dataLens) != 1 {
			d.fragments = d.fragments[:0] // discard pending fragmented packets
			return nil, 0, fmt.Errorf("a fragmented packet can only contain one AU")
		}

		if len(payload) < int(dataLens[0]) {
			d.fragments = d.fragments[:0] // discard pending fragmented packets
			return nil, 0, fmt.Errorf("payload is too short")
		}

		d.fragmentedSize += int(dataLens[0])
		if d.fragmentedSize > mpeg4audio.MaxAccessUnitSize {
			d.fragments = d.fragments[:0] // discard pending fragmented packets
			return nil, 0, fmt.Errorf("AU size (%d) is too big (maximum is %d)", d.fragmentedSize, mpeg4audio.MaxAccessUnitSize)
		}

		d.fragments = append(d.fragments, payload[:dataLens[0]])

		if !pkt.Header.Marker {
			return nil, 0, ErrMorePacketsNeeded
		}

		ret := make([]byte, d.fragmentedSize)
		n := 0
		for _, p := range d.fragments {
			n += copy(ret[n:], p)
		}
		aus = [][]byte{ret}

		d.fragments = d.fragments[:0]
	}

	aus, err = d.removeADTS(aus)
	if err != nil {
		return nil, 0, err
	}

	return aus, d.timeDecoder.Decode(pkt.Timestamp), nil
}

func (d *Decoder) readAUHeaders(buf []byte, headersLen int) ([]uint64, error) {
	firstRead := false

	count := 0
	for i := 0; i < headersLen; {
		if i == 0 {
			i += d.SizeLength
			i += d.IndexLength
		} else {
			i += d.SizeLength
			i += d.IndexDeltaLength
		}
		count++
	}

	dataLens := make([]uint64, count)

	pos := 0
	i := 0

	for headersLen > 0 {
		dataLen, err := bits.ReadBits(buf, &pos, d.SizeLength)
		if err != nil {
			return nil, err
		}
		headersLen -= d.SizeLength

		if !firstRead {
			firstRead = true
			if d.IndexLength > 0 {
				auIndex, err := bits.ReadBits(buf, &pos, d.IndexLength)
				if err != nil {
					return nil, err
				}
				headersLen -= d.IndexLength

				if auIndex != 0 {
					return nil, fmt.Errorf("AU-index different than zero is not supported")
				}
			}
		} else if d.IndexDeltaLength > 0 {
			auIndexDelta, err := bits.ReadBits(buf, &pos, d.IndexDeltaLength)
			if err != nil {
				return nil, err
			}
			headersLen -= d.IndexDeltaLength

			if auIndexDelta != 0 {
				return nil, fmt.Errorf("AU-index-delta different than zero is not supported")
			}
		}

		dataLens[i] = dataLen
		i++
	}

	return dataLens, nil
}

// some cameras wrap AUs into ADTS
func (d *Decoder) removeADTS(aus [][]byte) ([][]byte, error) {
	if !d.firstAUParsed {
		d.firstAUParsed = true

		if len(aus) == 1 && len(aus[0]) >= 2 {
			if aus[0][0] == 0xFF && (aus[0][1]&0xF0) == 0xF0 {
				var pkts mpeg4audio.ADTSPackets
				err := pkts.Unmarshal(aus[0])
				if err == nil && len(pkts) == 1 {
					d.adtsMode = true
					aus[0] = pkts[0].AU
				}
			}
		}
	} else if d.adtsMode {
		if len(aus) != 1 {
			return nil, fmt.Errorf("multiple AUs in ADTS mode are not supported")
		}

		var pkts mpeg4audio.ADTSPackets
		err := pkts.Unmarshal(aus[0])
		if err != nil {
			return nil, fmt.Errorf("unable to decode ADTS: %s", err)
		}

		if len(pkts) != 1 {
			return nil, fmt.Errorf("multiple ADTS packets are not supported")
		}

		aus[0] = pkts[0].AU
	}

	return aus, nil
}
