package rtph264

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
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

// Decoder is a RTP/H264 decoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc6184
type Decoder struct {
	// indicates the packetization mode.
	PacketizationMode int

	timeDecoder         *rtptime.Decoder
	firstPacketReceived bool
	fragmentsSize       int
	fragments           [][]byte
	annexBMode          bool

	// for DecodeUntilMarker()
	frameBuffer    [][]byte
	frameBufferLen int
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	if d.PacketizationMode >= 2 {
		return fmt.Errorf("PacketizationMode >= 2 is not supported")
	}

	d.timeDecoder = rtptime.NewDecoder(rtpClockRate)
	return nil
}

// Decode decodes NALUs from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	if len(pkt.Payload) < 1 {
		d.fragments = d.fragments[:0] // discard pending fragments
		return nil, 0, fmt.Errorf("payload is too short")
	}

	typ := h264.NALUType(pkt.Payload[0] & 0x1F)
	var nalus [][]byte

	switch typ {
	case h264.NALUTypeFUA:
		if len(pkt.Payload) < 2 {
			return nil, 0, fmt.Errorf("invalid FU-A packet (invalid size)")
		}

		start := pkt.Payload[1] >> 7
		end := (pkt.Payload[1] >> 6) & 0x01

		if start == 1 {
			d.fragments = d.fragments[:0] // discard pending fragments

			if end != 0 {
				return nil, 0, fmt.Errorf("invalid FU-A packet (can't contain both a start and end bit)")
			}

			nri := (pkt.Payload[0] >> 5) & 0x03
			typ := pkt.Payload[1] & 0x1F
			d.fragmentsSize = len(pkt.Payload[1:])
			d.fragments = append(d.fragments, []byte{(nri << 5) | typ}, pkt.Payload[2:])
			d.firstPacketReceived = true

			return nil, 0, ErrMorePacketsNeeded
		}

		if len(d.fragments) == 0 {
			if !d.firstPacketReceived {
				return nil, 0, ErrNonStartingPacketAndNoPrevious
			}

			return nil, 0, fmt.Errorf("invalid FU-A packet (non-starting)")
		}

		d.fragmentsSize += len(pkt.Payload[2:])
		if d.fragmentsSize > h264.MaxNALUSize {
			d.fragments = d.fragments[:0]
			return nil, 0, fmt.Errorf("NALU size (%d) is too big, maximum is %d", d.fragmentsSize, h264.MaxNALUSize)
		}

		d.fragments = append(d.fragments, pkt.Payload[2:])

		if end != 1 {
			return nil, 0, ErrMorePacketsNeeded
		}

		nalus = [][]byte{joinFragments(d.fragments, d.fragmentsSize)}
		d.fragments = d.fragments[:0]

	case h264.NALUTypeSTAPA:
		d.fragments = d.fragments[:0] // discard pending fragments

		payload := pkt.Payload[1:]

		for len(payload) > 0 {
			if len(payload) < 2 {
				return nil, 0, fmt.Errorf("invalid STAP-A packet (invalid size)")
			}

			size := uint16(payload[0])<<8 | uint16(payload[1])
			payload = payload[2:]

			// avoid final padding
			if size == 0 {
				break
			}

			if int(size) > len(payload) {
				return nil, 0, fmt.Errorf("invalid STAP-A packet (invalid size)")
			}

			nalus = append(nalus, payload[:size])
			payload = payload[size:]
		}

		if nalus == nil {
			return nil, 0, fmt.Errorf("STAP-A packet doesn't contain any NALU")
		}

		d.firstPacketReceived = true

	case h264.NALUTypeSTAPB, h264.NALUTypeMTAP16,
		h264.NALUTypeMTAP24, h264.NALUTypeFUB:
		d.fragments = d.fragments[:0] // discard pending fragments
		d.firstPacketReceived = true
		return nil, 0, fmt.Errorf("packet type not supported (%v)", typ)

	default:
		d.fragments = d.fragments[:0] // discard pending fragments
		d.firstPacketReceived = true
		nalus = [][]byte{pkt.Payload}
	}

	nalus, err := d.removeAnnexB(nalus)
	if err != nil {
		return nil, 0, err
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

	if (d.frameBufferLen + l) > h264.MaxNALUsPerGroup {
		d.frameBuffer = nil
		d.frameBufferLen = 0
		return nil, 0, fmt.Errorf("NALU count exceeds maximum allowed (%d)",
			h264.MaxNALUsPerGroup)
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

// some cameras / servers wrap NALUs into Annex-B
func (d *Decoder) removeAnnexB(nalus [][]byte) ([][]byte, error) {
	if len(nalus) == 1 {
		nalu := nalus[0]

		if !d.annexBMode && bytes.Contains(nalu, []byte{0x00, 0x00, 0x00, 0x01}) {
			d.annexBMode = true
		}

		if d.annexBMode {
			if !bytes.HasPrefix(nalu, []byte{0x00, 0x00, 0x00, 0x01}) {
				nalu = append([]byte{0x00, 0x00, 0x00, 0x01}, nalu...)
			}

			return h264.AnnexBUnmarshal(nalu)
		}
	}

	return nalus, nil
}
