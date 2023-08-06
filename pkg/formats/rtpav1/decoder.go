package rtpav1

import (
	"errors"
	"fmt"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/av1"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
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

// Decoder is a RTP/AV1 decoder.
// Specification: https://aomediacodec.github.io/av1-rtp-spec/
type Decoder struct {
	timeDecoder         *rtptime.Decoder
	firstPacketReceived bool
	fragmentsSize       int
	fragments           [][]byte

	// for DecodeUntilMarker()
	frameBuffer     [][]byte
	frameBufferLen  int
	frameBufferSize int
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	d.timeDecoder = rtptime.NewDecoder(90000)
	return nil
}

// Decode decodes OBUs from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	var av1header codecs.AV1Packet
	_, err := av1header.Unmarshal(pkt.Payload)
	if err != nil {
		d.fragments = d.fragments[:0] // discard pending fragments
		d.fragmentsSize = 0
		return nil, 0, fmt.Errorf("invalid header: %v", err)
	}

	for _, el := range av1header.OBUElements {
		if len(el) == 0 {
			return nil, 0, fmt.Errorf("invalid OBU fragment")
		}
	}

	if av1header.Z {
		if len(d.fragments) == 0 {
			if !d.firstPacketReceived {
				return nil, 0, ErrNonStartingPacketAndNoPrevious
			}

			return nil, 0, fmt.Errorf("received a subsequent fragment without previous fragments")
		}

		d.fragmentsSize += len(av1header.OBUElements[0])
		if d.fragmentsSize > av1.MaxTemporalUnitSize {
			d.fragments = d.fragments[:0]
			d.fragmentsSize = 0
			return nil, 0, fmt.Errorf("OBU size (%d) is too big, maximum is %d", d.fragmentsSize, av1.MaxTemporalUnitSize)
		}

		d.fragments = append(d.fragments, av1header.OBUElements[0])
		av1header.OBUElements = av1header.OBUElements[1:]
	}

	d.firstPacketReceived = true

	var obus [][]byte

	if len(av1header.OBUElements) > 0 {
		if d.fragmentsSize != 0 {
			obus = append(obus, joinFragments(d.fragments, d.fragmentsSize))
			d.fragments = d.fragments[:0]
			d.fragmentsSize = 0
		}

		if av1header.Y {
			elementCount := len(av1header.OBUElements)

			d.fragmentsSize += len(av1header.OBUElements[elementCount-1])
			if d.fragmentsSize > av1.MaxTemporalUnitSize {
				d.fragments = d.fragments[:0]
				d.fragmentsSize = 0
				return nil, 0, fmt.Errorf("OBU size (%d) is too big, maximum is %d", d.fragmentsSize, av1.MaxTemporalUnitSize)
			}

			d.fragments = append(d.fragments, av1header.OBUElements[elementCount-1])
			av1header.OBUElements = av1header.OBUElements[:elementCount-1]
		}

		obus = append(obus, av1header.OBUElements...)
	} else if !av1header.Y {
		obus = append(obus, joinFragments(d.fragments, d.fragmentsSize))
		d.fragments = d.fragments[:0]
		d.fragmentsSize = 0
	}

	if len(obus) == 0 {
		return nil, 0, ErrMorePacketsNeeded
	}

	return obus, d.timeDecoder.Decode(pkt.Timestamp), nil
}

// DecodeUntilMarker decodes OBUs from a RTP packet and puts them in a buffer.
// When a packet has the marker flag (meaning that all OBUs with the same PTS have
// been received), the buffer is returned.
func (d *Decoder) DecodeUntilMarker(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	obus, pts, err := d.Decode(pkt)
	if err != nil {
		return nil, 0, err
	}
	l := len(obus)

	if (d.frameBufferLen + l) > av1.MaxOBUsPerTemporalUnit {
		d.frameBuffer = nil
		d.frameBufferLen = 0
		d.frameBufferSize = 0
		return nil, 0, fmt.Errorf("OBU count exceeds maximum allowed (%d)",
			av1.MaxOBUsPerTemporalUnit)
	}

	addSize := 0

	for _, obu := range obus {
		addSize += len(obu)
	}

	if (d.frameBufferSize + addSize) > av1.MaxTemporalUnitSize {
		d.frameBuffer = nil
		d.frameBufferLen = 0
		d.frameBufferSize = 0
		return nil, 0, fmt.Errorf("temporal unit size (%d) is too big, maximum is %d",
			d.frameBufferSize+addSize, av1.MaxOBUsPerTemporalUnit)
	}

	d.frameBuffer = append(d.frameBuffer, obus...)
	d.frameBufferLen += l
	d.frameBufferSize += addSize

	if !pkt.Marker {
		return nil, 0, ErrMorePacketsNeeded
	}

	ret := d.frameBuffer

	// do not reuse frameBuffer to avoid race conditions
	d.frameBuffer = nil
	d.frameBufferLen = 0
	d.frameBufferSize = 0

	return ret, pts, nil
}
