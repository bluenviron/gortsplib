package rtpsimpleaudio

import (
	"time"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/rtptimedec"
)

// Decoder is a RTP/simple audio decoder.
type Decoder struct {
	SampleRate int

	timeDecoder *rtptimedec.Decoder
}

// Init initializes the decoder.
func (d *Decoder) Init() {
	d.timeDecoder = rtptimedec.New(d.SampleRate)
}

// Decode decodes an audio frame from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, time.Duration, error) {
	return pkt.Payload, d.timeDecoder.Decode(pkt.Timestamp), nil
}
