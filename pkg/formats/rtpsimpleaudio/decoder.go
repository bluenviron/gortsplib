package rtpsimpleaudio

import (
	"time"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
)

// Decoder is a RTP/simple audio decoder.
type Decoder struct {
	SampleRate int

	timeDecoder *rtptime.Decoder
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	d.timeDecoder = rtptime.NewDecoder(d.SampleRate)
	return nil
}

// Decode decodes an audio frame from a RTP packet.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, time.Duration, error) {
	return pkt.Payload, d.timeDecoder.Decode(pkt.Timestamp), nil
}
