package rtpopus

import (
	"time"

	"github.com/pion/rtp"
	"github.com/pion/rtp/codecs"

	"github.com/aler9/gortsplib/pkg/rtptimedec"
)

// Decoder is a RTP/Opus decoder.
type Decoder struct {
	// sample rate of input packets.
	SampleRate int

	cop         codecs.OpusPacket
	timeDecoder *rtptimedec.Decoder
}

// Init initializes the decoder.
func (d *Decoder) Init() {
	d.timeDecoder = rtptimedec.New(d.SampleRate)
}

// Decode decodes a Opus packet from a RTP/Opus packet.
// It returns the Opus packet and its PTS.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, time.Duration, error) {
	_, err := d.cop.Unmarshal(pkt.Payload)
	return d.cop.Payload, d.timeDecoder.Decode(pkt.Timestamp), err
}
