package rtpmpeg4audio

import (
	"errors"
	"fmt"

	"github.com/bluenviron/mediacommon/v2/pkg/bits"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

func joinFragments(fragments [][]byte, size int) []byte {
	ret := make([]byte, size)
	n := 0
	for _, p := range fragments {
		n += copy(ret[n:], p)
	}
	return ret
}

func readAUHeaders(
	buf []byte,
	headersLen int,
	sizeLength int,
	indexLength int,
	indexDeltaLength int,
) ([]uint64, error) {
	firstHeaderLen := sizeLength + indexLength
	otherHeaderLen := sizeLength + indexDeltaLength

	if headersLen < firstHeaderLen {
		return nil, fmt.Errorf("invalid AU-headers-length")
	}

	count := 1
	remaining := headersLen - firstHeaderLen
	for remaining > 0 {
		if remaining < otherHeaderLen {
			return nil, fmt.Errorf("invalid AU-headers-length")
		}

		remaining -= otherHeaderLen
		count++
	}

	dataLens := make([]uint64, count)
	pos := 0

	for i := range dataLens {
		dataLen, err := bits.ReadBits(buf, &pos, sizeLength)
		if err != nil {
			return nil, err
		}

		if dataLen == 0 {
			return nil, fmt.Errorf("invalid data length")
		}

		if i == 0 {
			if indexLength > 0 {
				var auIndex uint64
				auIndex, err = bits.ReadBits(buf, &pos, indexLength)
				if err != nil {
					return nil, err
				}

				if auIndex != 0 {
					return nil, fmt.Errorf("AU-index different than zero is not supported")
				}
			}
		} else if indexDeltaLength > 0 {
			var auIndexDelta uint64
			auIndexDelta, err = bits.ReadBits(buf, &pos, indexDeltaLength)
			if err != nil {
				return nil, err
			}

			if auIndexDelta != 0 {
				return nil, fmt.Errorf("AU-index-delta different than zero is not supported")
			}
		}

		dataLens[i] = dataLen
	}

	return dataLens, nil
}

// Decoder is a RTP/MPEG-4 Audio decoder.
// Specification: RFC3640
type Decoder struct {
	// The number of bits in which the AU-size field is encoded in the AU-header.
	SizeLength int

	// The number of bits in which the AU-Index is encoded in the first AU-header.
	IndexLength int

	// The number of bits in which the AU-Index-delta field is encoded in any non-first AU-header.
	IndexDeltaLength int

	firstAUParsed      bool
	adtsMode           bool
	fragments          [][]byte
	fragmentsSize      int
	fragmentAUSize     int
	fragmentNextSeqNum uint16
	fragmentTimestamp  uint32
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	if d.SizeLength == 0 {
		return fmt.Errorf("invalid AU-size length")
	}

	return nil
}

func (d *Decoder) resetFragments() {
	d.fragments = d.fragments[:0]
	d.fragmentsSize = 0
	d.fragmentAUSize = 0
	d.fragmentNextSeqNum = 0
	d.fragmentTimestamp = 0
}

// Decode decodes access units from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([][]byte, error) {
	if len(pkt.Payload) < 2 {
		d.resetFragments()
		return nil, fmt.Errorf("payload is too short")
	}

	// AU-headers-length (16 bits)
	headersLen := int(uint16(pkt.Payload[0])<<8 | uint16(pkt.Payload[1]))
	if headersLen == 0 {
		d.resetFragments()
		return nil, fmt.Errorf("invalid AU-headers-length")
	}
	payload := pkt.Payload[2:]

	// AU-headers
	dataLens, err := readAUHeaders(payload, headersLen, d.SizeLength, d.IndexLength, d.IndexDeltaLength)
	if err != nil {
		d.resetFragments()
		return nil, err
	}

	pos := headersLen / 8
	if (headersLen % 8) != 0 {
		pos++
	}
	if len(payload) < pos {
		d.resetFragments()
		return nil, fmt.Errorf("payload is too short")
	}
	payload = payload[pos:]

	var aus [][]byte

	if d.fragmentAUSize == 0 {
		d.resetFragments()

		if pkt.Marker {
			aus = make([][]byte, len(dataLens))
			for i, dataLen := range dataLens {
				if len(payload) < int(dataLen) {
					return nil, fmt.Errorf("payload is too short")
				}

				aus[i] = payload[:dataLen]
				payload = payload[dataLen:]
			}

			if len(payload) != 0 {
				return nil, fmt.Errorf("payload has invalid size")
			}
		} else {
			if len(dataLens) != 1 {
				return nil, fmt.Errorf("a fragmented packet can only contain one AU")
			}

			auSize := int(dataLens[0])
			if auSize > mpeg4audio.MaxAccessUnitSize {
				return nil, fmt.Errorf("access unit size (%d) is too big, maximum is %d",
					auSize, mpeg4audio.MaxAccessUnitSize)
			}

			if len(payload) == 0 || len(payload) >= auSize {
				return nil, fmt.Errorf("invalid fragmented access unit")
			}

			d.fragmentsSize = len(payload)
			d.fragmentAUSize = auSize
			d.fragments = append(d.fragments, payload)
			d.fragmentNextSeqNum = pkt.SequenceNumber + 1
			d.fragmentTimestamp = pkt.Timestamp
			return nil, ErrMorePacketsNeeded
		}
	} else {
		// we are decoding a fragmented AU
		if len(dataLens) != 1 {
			d.resetFragments()
			return nil, fmt.Errorf("a fragmented packet can only contain one AU")
		}

		if int(dataLens[0]) != d.fragmentAUSize {
			d.resetFragments()
			return nil, fmt.Errorf("AU size differs from previous fragment")
		}

		if pkt.SequenceNumber != d.fragmentNextSeqNum {
			d.resetFragments()
			return nil, fmt.Errorf("discarding frame since a RTP packet is missing")
		}

		if pkt.Timestamp != d.fragmentTimestamp {
			d.resetFragments()
			return nil, fmt.Errorf("RTP timestamp differs from previous fragment")
		}

		d.fragmentsSize += len(payload)
		if d.fragmentsSize > d.fragmentAUSize {
			d.resetFragments()
			return nil, fmt.Errorf("access unit size exceeds declared size")
		}

		d.fragments = append(d.fragments, payload)
		d.fragmentNextSeqNum++

		if !pkt.Marker {
			if d.fragmentsSize == d.fragmentAUSize {
				d.resetFragments()
				return nil, fmt.Errorf("invalid fragmented access unit")
			}

			return nil, ErrMorePacketsNeeded
		}

		if d.fragmentsSize != d.fragmentAUSize {
			d.resetFragments()
			return nil, fmt.Errorf("access unit size does not match declared size")
		}

		aus = [][]byte{joinFragments(d.fragments, d.fragmentsSize)}
		d.resetFragments()
	}

	return d.removeADTS(aus)
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
			return nil, fmt.Errorf("unable to decode ADTS: %w", err)
		}

		if len(pkts) != 1 {
			return nil, fmt.Errorf("multiple ADTS packets are not supported")
		}

		aus[0] = pkts[0].AU
	}

	return aus, nil
}
