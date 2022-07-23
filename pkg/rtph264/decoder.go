package rtph264

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/pkg/h264"
	"github.com/aler9/gortsplib/pkg/rtptimedec"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

// ErrNonStartingPacketAndNoPrevious is returned when we received a non-starting
// packet of a fragmented NALU and we didn't received anything before.
// It's normal to receive this when we are decoding a stream that has been already
// running for some time.
var ErrNonStartingPacketAndNoPrevious = errors.New(
	"received a non-starting FU-A packet without any previous FU-A starting packet")

// Decoder is a RTP/H264 decoder.
type Decoder struct {
	timeDecoder         *rtptimedec.Decoder
	firstPacketReceived bool
	fragmentedMode      bool
	fragmentedParts     [][]byte
	fragmentedSize      int
	firstNALUParsed     bool
	annexBMode          bool

	// for DecodeUntilMarker()
	naluBuffer [][]byte
}

// Init initializes the decoder
func (d *Decoder) Init() {
	d.timeDecoder = rtptimedec.New(rtpClockRate)
}

// Decode decodes NALUs from a RTP/H264 packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	if !d.fragmentedMode {
		if len(pkt.Payload) < 1 {
			return nil, 0, fmt.Errorf("payload is too short")
		}

		typ := naluType(pkt.Payload[0] & 0x1F)

		switch typ {
		case naluTypeSTAPA:
			var nalus [][]byte
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

			if len(nalus) == 0 {
				return nil, 0, fmt.Errorf("STAP-A packet doesn't contain any NALU")
			}

			d.firstPacketReceived = true

			var err error
			nalus, err = d.finalize(nalus)
			if err != nil {
				return nil, 0, err
			}

			return nalus, d.timeDecoder.Decode(pkt.Timestamp), nil

		case naluTypeFUA: // first packet of a fragmented NALU
			if len(pkt.Payload) < 2 {
				return nil, 0, fmt.Errorf("invalid FU-A packet (invalid size)")
			}

			start := pkt.Payload[1] >> 7
			if start != 1 {
				if !d.firstPacketReceived {
					return nil, 0, ErrNonStartingPacketAndNoPrevious
				}
				return nil, 0, fmt.Errorf("invalid FU-A packet (non-starting)")
			}

			end := (pkt.Payload[1] >> 6) & 0x01
			if end != 0 {
				return nil, 0, fmt.Errorf("invalid FU-A packet (can't contain both a start and end bit)")
			}

			nri := (pkt.Payload[0] >> 5) & 0x03
			typ := pkt.Payload[1] & 0x1F
			d.fragmentedSize = len(pkt.Payload) - 1
			d.fragmentedParts = append(d.fragmentedParts, []byte{(nri << 5) | typ})
			d.fragmentedParts = append(d.fragmentedParts, pkt.Payload[2:])
			d.fragmentedMode = true

			d.firstPacketReceived = true
			return nil, 0, ErrMorePacketsNeeded

		case naluTypeSTAPB, naluTypeMTAP16,
			naluTypeMTAP24, naluTypeFUB:
			return nil, 0, fmt.Errorf("packet type not supported (%v)", typ)
		}

		nalus := [][]byte{pkt.Payload}

		d.firstPacketReceived = true

		var err error
		nalus, err = d.finalize(nalus)
		if err != nil {
			return nil, 0, err
		}

		return nalus, d.timeDecoder.Decode(pkt.Timestamp), nil
	}

	// we are decoding a fragmented NALU

	if len(pkt.Payload) < 2 {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("invalid FU-A packet (invalid size)")
	}

	typ := naluType(pkt.Payload[0] & 0x1F)
	if typ != naluTypeFUA {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("expected FU-A packet, got %s packet", typ)
	}

	start := pkt.Payload[1] >> 7
	if start == 1 {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("invalid FU-A packet (decoded two starting packets in a row)")
	}

	d.fragmentedSize += len(pkt.Payload[2:])
	if d.fragmentedSize > h264.MaxNALUSize {
		d.fragmentedParts = d.fragmentedParts[:0]
		d.fragmentedMode = false
		return nil, 0, fmt.Errorf("NALU size (%d) is too big (maximum is %d)", d.fragmentedSize, h264.MaxNALUSize)
	}

	d.fragmentedParts = append(d.fragmentedParts, pkt.Payload[2:])

	end := (pkt.Payload[1] >> 6) & 0x01
	if end != 1 {
		return nil, 0, ErrMorePacketsNeeded
	}

	ret := make([]byte, d.fragmentedSize)
	n := 0
	for _, p := range d.fragmentedParts {
		n += copy(ret[n:], p)
	}
	nalus := [][]byte{ret}

	d.fragmentedParts = d.fragmentedParts[:0]
	d.fragmentedMode = false

	var err error
	nalus, err = d.finalize(nalus)
	if err != nil {
		return nil, 0, err
	}

	return nalus, d.timeDecoder.Decode(pkt.Timestamp), nil
}

// DecodeUntilMarker decodes NALUs from a RTP/H264 packet and puts them in a buffer.
// When a packet has the marker flag (meaning that all the NALUs with the same PTS have
// been received), the buffer is returned.
func (d *Decoder) DecodeUntilMarker(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	nalus, pts, err := d.Decode(pkt)
	if err != nil {
		return nil, 0, err
	}

	if (len(d.naluBuffer) + len(nalus)) >= h264.MaxNALUsPerGroup {
		return nil, 0, fmt.Errorf("number of NALUs contained inside a single group (%d) is too big (maximum is %d)",
			len(d.naluBuffer)+len(nalus), h264.MaxNALUsPerGroup)
	}

	d.naluBuffer = append(d.naluBuffer, nalus...)

	if !pkt.Marker {
		return nil, 0, ErrMorePacketsNeeded
	}

	ret := d.naluBuffer
	d.naluBuffer = d.naluBuffer[:0]

	return ret, pts, nil
}

func (d *Decoder) finalize(nalus [][]byte) ([][]byte, error) {
	// some cameras / servers wrap NALUs into Annex-B
	if !d.firstNALUParsed {
		d.firstNALUParsed = true

		if len(nalus) == 1 {
			nalu := nalus[0]

			i := bytes.Index(nalu, []byte{0x00, 0x00, 0x00, 0x01})
			if i >= 0 {
				d.annexBMode = true

				if !bytes.HasPrefix(nalu, []byte{0x00, 0x00, 0x00, 0x01}) {
					nalu = append([]byte{0x00, 0x00, 0x00, 0x01}, nalu...)
				}

				return h264.AnnexBUnmarshal(nalu)
			}
		}
	} else if d.annexBMode {
		if len(nalus) != 1 {
			return nil, fmt.Errorf("multiple NALUs in Annex-B mode are not supported")
		}

		nalu := nalus[0]

		if !bytes.HasPrefix(nalu, []byte{0x00, 0x00, 0x00, 0x01}) {
			nalu = append([]byte{0x00, 0x00, 0x00, 0x01}, nalu...)
		}

		return h264.AnnexBUnmarshal(nalu)
	}

	return nalus, nil
}
