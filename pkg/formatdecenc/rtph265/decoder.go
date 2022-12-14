package rtph265

import (
	"errors"
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/h265"
	"github.com/aler9/gortsplib/v2/pkg/rtptimedec"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

// ErrNonStartingPacketAndNoPrevious is returned when we received a non-starting
// packet of a fragmented NALU and we didn't received anything before.
// It's normal to receive this when we are decoding a stream that has been already
// running for some time.
var ErrNonStartingPacketAndNoPrevious = errors.New(
	"received a non-starting fragmentation unit without any previous fragmentation units")

// Decoder is a RTP/H265 decoder.
type Decoder struct {
	// indicates that NALUs have an additional field that specifies the decoding order.
	MaxDONDiff int

	timeDecoder         *rtptimedec.Decoder
	firstPacketReceived bool
	fragmentedSize      int
	fragments           [][]byte
}

// Init initializes the decoder.
func (d *Decoder) Init() {
	d.timeDecoder = rtptimedec.New(rtpClockRate)
}

// Decode decodes NALUs from a RTP/H265 packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	if d.MaxDONDiff != 0 {
		return nil, 0, fmt.Errorf("MaxDONDiff != 0 is not supported (yet)")
	}

	if len(pkt.Payload) < 2 {
		d.fragments = d.fragments[:0] // discard pending fragmented packets
		return nil, 0, fmt.Errorf("payload is too short")
	}

	typ := h265.NALUType((pkt.Payload[0] >> 1) & 0b111111)
	var nalus [][]byte

	switch typ {
	case h265.NALUTypeAggregationUnit:
		d.fragments = d.fragments[:0] // discard pending fragmented packets

		payload := pkt.Payload[2:]

		for len(payload) > 0 {
			if len(payload) < 2 {
				return nil, 0, fmt.Errorf("invalid aggregation unit (invalid size)")
			}

			size := uint16(payload[0])<<8 | uint16(payload[1])
			payload = payload[2:]

			if size == 0 {
				break
			}

			if int(size) > len(payload) {
				return nil, 0, fmt.Errorf("invalid aggregation unit (invalid size)")
			}

			nalus = append(nalus, payload[:size])
			payload = payload[size:]
		}

		if nalus == nil {
			return nil, 0, fmt.Errorf("aggregation unit doesn't contain any NALU")
		}

		d.firstPacketReceived = true

	case h265.NALUTypeFragmentationUnit:
		if len(pkt.Payload) < 3 {
			d.fragments = d.fragments[:0] // discard pending fragmented packets
			return nil, 0, fmt.Errorf("payload is too short")
		}

		start := pkt.Payload[2] >> 7
		end := (pkt.Payload[2] >> 6) & 0x01

		if start == 1 {
			d.fragments = d.fragments[:0] // discard pending fragmented packets

			if end != 0 {
				return nil, 0, fmt.Errorf("invalid fragmentation unit (can't contain both a start and end bit)")
			}

			typ := pkt.Payload[2] & 0b111111
			head := uint16(pkt.Payload[0]&0b10000001)<<8 | uint16(typ)<<9 | uint16(pkt.Payload[1])
			d.fragmentedSize = len(pkt.Payload[1:])
			d.fragments = append(d.fragments, []byte{byte(head >> 8), byte(head)}, pkt.Payload[3:])
			d.firstPacketReceived = true

			return nil, 0, ErrMorePacketsNeeded
		}

		if len(d.fragments) == 0 {
			if !d.firstPacketReceived {
				return nil, 0, ErrNonStartingPacketAndNoPrevious
			}

			return nil, 0, fmt.Errorf("invalid fragmentation unit (non-starting)")
		}

		d.fragmentedSize += len(pkt.Payload[3:])
		if d.fragmentedSize > h265.MaxNALUSize {
			d.fragments = d.fragments[:0]
			return nil, 0, fmt.Errorf("NALU size (%d) is too big (maximum is %d)", d.fragmentedSize, h265.MaxNALUSize)
		}

		d.fragments = append(d.fragments, pkt.Payload[3:])

		if end != 1 {
			return nil, 0, ErrMorePacketsNeeded
		}

		nalu := make([]byte, d.fragmentedSize)
		pos := 0

		for _, frag := range d.fragments {
			pos += copy(nalu[pos:], frag)
		}

		d.fragments = d.fragments[:0]
		nalus = [][]byte{nalu}

	case h265.NALUTypePACI:
		d.fragments = d.fragments[:0] // discard pending fragmented packets
		d.firstPacketReceived = true
		return nil, 0, fmt.Errorf("PACI packets are not supported (yet)")

	default:
		d.fragments = d.fragments[:0] // discard pending fragmented packets
		d.firstPacketReceived = true
		nalus = [][]byte{pkt.Payload}
	}

	return nalus, d.timeDecoder.Decode(pkt.Timestamp), nil
}
