package rtpvp8

import (
	"errors"
	"fmt"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/vp8"
	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"
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

// Decoder is a RTP/VP8 decoder.
// Specification: RFC7741
type Decoder struct {
	firstPacketReceived bool
	frameBuffer         [][]byte
	frameBufferSize     int
	frameNextSeqNum     uint16
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	return nil
}

func (d *Decoder) resetFrameBuffer() {
	d.frameBuffer = nil // do not reuse frameBuffer to avoid race conditions
	d.frameBufferSize = 0
	d.frameNextSeqNum = 0
}

func (d *Decoder) decodeFrameChunk(pkt *rtp.Packet) ([]byte, error) {
	var vpkt codecs.VP8Packet
	_, err := vpkt.Unmarshal(pkt.Payload)
	if err != nil {
		d.resetFrameBuffer()
		return nil, err
	}

	if len(vpkt.Payload) == 0 {
		d.resetFrameBuffer()
		return nil, fmt.Errorf("payload is empty")
	}

	// VP8 frame restarts are identified by a packet that starts partition 0.
	// Any previously buffered frame must be discarded in this case.
	if vpkt.S == 1 && vpkt.PID == 0 {
		d.resetFrameBuffer()
		d.firstPacketReceived = true
		d.frameNextSeqNum = pkt.SequenceNumber + 1
		return vpkt.Payload, nil
	}

	if d.frameBufferSize == 0 {
		if !d.firstPacketReceived {
			return nil, ErrNonStartingPacketAndNoPrevious
		}

		return nil, fmt.Errorf("received a non-starting fragment")
	}

	if pkt.SequenceNumber != d.frameNextSeqNum {
		d.resetFrameBuffer()
		return nil, fmt.Errorf("discarding frame since a RTP packet is missing")
	}

	d.firstPacketReceived = true
	d.frameNextSeqNum++
	return vpkt.Payload, nil
}

// Decode decodes a VP8 frame from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, error) {
	chunk, err := d.decodeFrameChunk(pkt)
	if err != nil {
		return nil, err
	}

	newFrameBufferSize := d.frameBufferSize + len(chunk)
	if newFrameBufferSize > vp8.MaxFrameSize {
		errSize := newFrameBufferSize
		d.resetFrameBuffer()
		return nil, fmt.Errorf("frame size (%d) is too big, maximum is %d",
			errSize, vp8.MaxFrameSize)
	}

	d.frameBuffer = append(d.frameBuffer, chunk)
	d.frameBufferSize = newFrameBufferSize

	if !pkt.Marker {
		return nil, ErrMorePacketsNeeded
	}

	frame := joinFragments(d.frameBuffer, d.frameBufferSize)
	d.resetFrameBuffer()

	return frame, nil
}
