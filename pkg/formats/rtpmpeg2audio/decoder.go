package rtpmpeg2audio

import (
	"errors"
	"fmt"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg2audio"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

// Decoder is a RTP/MPEG2-audio decoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc2250
type Decoder struct {
	timeDecoder       *rtptime.Decoder
	fragments         [][]byte
	fragmentedSize    int
	fragmentsExpected int
}

// Init initializes the decoder.
func (d *Decoder) Init() {
	d.timeDecoder = rtptime.NewDecoder(90000)
}

// Decode decodes frames from a RTP/MPEG2-audio packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([][]byte, time.Duration, error) {
	if len(pkt.Payload) < 5 {
		return nil, 0, fmt.Errorf("payload is too short")
	}

	mbz := uint16(pkt.Payload[0])<<8 | uint16(pkt.Payload[1])
	if mbz != 0 {
		return nil, 0, fmt.Errorf("invalid MBZ: %v", mbz)
	}

	offset := uint16(pkt.Payload[2])<<8 | uint16(pkt.Payload[3])

	var frames [][]byte

	if len(d.fragments) == 0 {
		if offset != 0 {
			return nil, 0, fmt.Errorf("invalid offset")
		}

		buf := pkt.Payload[4:]
		for {
			var h mpeg2audio.FrameHeader
			err := h.Unmarshal(buf)
			if err != nil {
				return nil, 0, err
			}

			fl := h.FrameLen()
			bl := len(buf)
			if bl >= fl {
				frames = append(frames, buf[:fl])
				buf = buf[fl:]
				if len(buf) == 0 {
					break
				}
			} else {
				if len(frames) != 0 {
					return nil, 0, fmt.Errorf("invalid packet")
				}

				d.fragments = append(d.fragments, buf)
				d.fragmentedSize = bl
				d.fragmentsExpected = fl - bl
				return nil, 0, ErrMorePacketsNeeded
			}
		}
	} else {
		if int(offset) != d.fragmentedSize {
			d.fragments = d.fragments[:0] // discard pending fragmented packets
			return nil, 0, fmt.Errorf("unexpected offset %v, expected %v", offset, d.fragmentedSize)
		}

		bl := len(pkt.Payload[4:])
		d.fragmentedSize += bl
		d.fragmentsExpected -= bl

		if d.fragmentsExpected < 0 {
			d.fragments = d.fragments[:0] // discard pending fragmented packets
			return nil, 0, fmt.Errorf("fragment is too big")
		}

		d.fragments = append(d.fragments, pkt.Payload[4:])

		if d.fragmentsExpected > 0 {
			return nil, 0, ErrMorePacketsNeeded
		}

		ret := make([]byte, d.fragmentedSize)
		n := 0
		for _, p := range d.fragments {
			n += copy(ret[n:], p)
		}
		frames = [][]byte{ret}

		d.fragments = d.fragments[:0]
	}

	return frames, d.timeDecoder.Decode(pkt.Timestamp), nil
}
