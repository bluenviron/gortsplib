package rtpvp9

import (
	"errors"
	"fmt"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/vp9"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
)

// ErrMorePacketsNeeded is returned when more packets are needed.
var ErrMorePacketsNeeded = errors.New("need more packets")

// ErrNonStartingPacketAndNoPrevious is returned when we received a non-starting
// packet of a fragmented frame and we didn't received anything before.
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

// Decoder is a RTP/VP9 decoder.
// Specification: https://datatracker.ietf.org/doc/html/draft-ietf-payload-vp9-16
type Decoder struct {
	timeDecoder         *rtptime.Decoder
	firstPacketReceived bool
	fragmentsSize       int
	fragments           [][]byte
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	d.timeDecoder = rtptime.NewDecoder(rtpClockRate)
	return nil
}

// Decode decodes a VP9 frame from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, time.Duration, error) {
	var vpkt codecs.VP9Packet
	_, err := vpkt.Unmarshal(pkt.Payload)
	if err != nil {
		d.fragments = d.fragments[:0] // discard pending fragments
		return nil, 0, err
	}

	var frame []byte

	if vpkt.B {
		d.fragments = d.fragments[:0] // discard pending fragments
		d.firstPacketReceived = true

		if !vpkt.E {
			d.fragmentsSize = len(vpkt.Payload)
			d.fragments = append(d.fragments, vpkt.Payload)
			return nil, 0, ErrMorePacketsNeeded
		}

		frame = vpkt.Payload
	} else {
		if len(d.fragments) == 0 {
			if !d.firstPacketReceived {
				return nil, 0, ErrNonStartingPacketAndNoPrevious
			}

			return nil, 0, fmt.Errorf("received a non-starting fragment")
		}

		d.fragmentsSize += len(vpkt.Payload)

		if d.fragmentsSize > vp9.MaxFrameSize {
			d.fragments = d.fragments[:0] // discard pending fragments
			return nil, 0, fmt.Errorf("frame size (%d) is too big, maximum is %d", d.fragmentsSize, vp9.MaxFrameSize)
		}

		d.fragments = append(d.fragments, vpkt.Payload)

		if !vpkt.E {
			return nil, 0, ErrMorePacketsNeeded
		}

		frame = joinFragments(d.fragments, d.fragmentsSize)
		d.fragments = d.fragments[:0]
	}

	return frame, d.timeDecoder.Decode(pkt.Timestamp), nil
}
