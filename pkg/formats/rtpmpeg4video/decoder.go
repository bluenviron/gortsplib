package rtpmpeg4video

import (
	"errors"
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

const (
	maxFrameSize = 1 * 1024 * 1024
)

func joinFragments(fragments [][]byte, size int) []byte {
	ret := make([]byte, size)
	n := 0
	for _, p := range fragments {
		n += copy(ret[n:], p)
	}
	return ret
}

// Decoder is a RTP/MPEG-4 Video decoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc6416
type Decoder struct {
	timeDecoder    *rtptime.Decoder
	fragments      [][]byte
	fragmentedSize int
}

// Init initializes the decoder.
func (d *Decoder) Init() {
	d.timeDecoder = rtptime.NewDecoder(90000)
}

// Decode decodes a frame from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, time.Duration, error) {
	var frame []byte

	if len(d.fragments) == 0 {
		if pkt.Marker {
			frame = pkt.Payload
		} else {
			d.fragmentedSize = len(pkt.Payload)
			d.fragments = append(d.fragments, pkt.Payload)
			return nil, 0, ErrMorePacketsNeeded
		}
	} else {
		d.fragmentedSize += len(pkt.Payload)
		if d.fragmentedSize > maxFrameSize {
			d.fragments = d.fragments[:0] // discard pending fragmented packets
			return nil, 0, fmt.Errorf("frame size (%d) is too big (maximum is %d)", d.fragmentedSize, maxFrameSize)
		}

		d.fragments = append(d.fragments, pkt.Payload)

		if !pkt.Marker {
			return nil, 0, ErrMorePacketsNeeded
		}

		frame = joinFragments(d.fragments, d.fragmentedSize)
		d.fragments = d.fragments[:0]
	}

	return frame, d.timeDecoder.Decode(pkt.Timestamp), nil
}
