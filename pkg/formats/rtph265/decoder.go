package rtph265

import (
	"errors"
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
	"github.com/bluenviron/mediacommon/pkg/codecs/h265"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

// ErrNonStartingPacketAndNoPrevious is returned when we received a non-starting
// packet of a fragmented NALU and we didn't received anything before.
// It's normal to receive this when decoding a stream that has been already
// running for some time.
var ErrNonStartingPacketAndNoPrevious = errors.New(
	"received a non-starting fragment without any previous starting fragment")

func joinFragments(fragments [][]byte, size int) []byte {
	ret := make([]byte, size)
	n := 0
	for _, p := range fragments {
		n += copy(ret[n:], p)
	}
	return ret
}

// Decoder is a RTP/H265 decoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc7798
type Decoder struct {
	// indicates that NALUs have an additional field that specifies the decoding order.
	MaxDONDiff int

	timeDecoder         *rtptime.Decoder
	firstPacketReceived bool
	fragmentsSize       int
	fragments           [][]byte

	// for DecodeUntilMarker()
	frameBuffer    [][]byte
	frameBufferLen int
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	if d.MaxDONDiff != 0 {
		return fmt.Errorf("MaxDONDiff != 0 is not supported (yet)")
	}

	d.timeDecoder = rtptime.NewDecoder(rtpClockRate)
	return nil
}

// Decode decodes NALUs from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	if len(pkt.Payload) < 2 {
		d.fragments = d.fragments[:0] // discard pending fragments
		return nil, 0, fmt.Errorf("payload is too short")
	}

	typ := h265.NALUType((pkt.Payload[0] >> 1) & 0b111111)
	var nalus [][]byte

	switch typ {
	case h265.NALUType_AggregationUnit:
		d.fragments = d.fragments[:0] // discard pending fragments

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

	case h265.NALUType_FragmentationUnit:
		if len(pkt.Payload) < 3 {
			d.fragments = d.fragments[:0] // discard pending fragments
			return nil, 0, fmt.Errorf("payload is too short")
		}

		start := pkt.Payload[2] >> 7
		end := (pkt.Payload[2] >> 6) & 0x01

		if start == 1 {
			d.fragments = d.fragments[:0] // discard pending fragments

			if end != 0 {
				return nil, 0, fmt.Errorf("invalid fragmentation unit (can't contain both a start and end bit)")
			}

			typ := pkt.Payload[2] & 0b111111
			head := uint16(pkt.Payload[0]&0b10000001)<<8 | uint16(typ)<<9 | uint16(pkt.Payload[1])
			d.fragmentsSize = len(pkt.Payload[1:])
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

		d.fragmentsSize += len(pkt.Payload[3:])
		if d.fragmentsSize > h265.MaxNALUSize {
			d.fragments = d.fragments[:0]
			return nil, 0, fmt.Errorf("NALU size (%d) is too big, maximum is %d", d.fragmentsSize, h265.MaxNALUSize)
		}

		d.fragments = append(d.fragments, pkt.Payload[3:])

		if end != 1 {
			return nil, 0, ErrMorePacketsNeeded
		}

		nalus = [][]byte{joinFragments(d.fragments, d.fragmentsSize)}
		d.fragments = d.fragments[:0]

	case h265.NALUType_PACI:
		d.fragments = d.fragments[:0] // discard pending fragments
		d.firstPacketReceived = true
		return nil, 0, fmt.Errorf("PACI packets are not supported (yet)")

	default:
		d.fragments = d.fragments[:0] // discard pending fragments
		d.firstPacketReceived = true
		nalus = [][]byte{pkt.Payload}
	}

	return nalus, d.timeDecoder.Decode(pkt.Timestamp), nil
}

// DecodeUntilMarker decodes NALUs from a RTP packet and puts them in a buffer.
// When a packet has the marker flag (meaning that all the NALUs with the same PTS have
// been received), the buffer is returned.
func (d *Decoder) DecodeUntilMarker(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	nalus, pts, err := d.Decode(pkt)
	if err != nil {
		return nil, 0, err
	}
	l := len(nalus)

	if (d.frameBufferLen + l) > h265.MaxNALUsPerGroup {
		d.frameBuffer = nil
		d.frameBufferLen = 0
		return nil, 0, fmt.Errorf("NALU count exceeds maximum allowed (%d)",
			h265.MaxNALUsPerGroup)
	}

	d.frameBuffer = append(d.frameBuffer, nalus...)
	d.frameBufferLen += l

	if !pkt.Marker {
		return nil, 0, ErrMorePacketsNeeded
	}

	ret := d.frameBuffer

	// do not reuse frameBuffer to avoid race conditions
	d.frameBuffer = nil
	d.frameBufferLen = 0

	return ret, pts, nil
}
