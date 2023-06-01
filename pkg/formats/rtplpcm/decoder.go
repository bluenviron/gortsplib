package rtplpcm

import (
	"fmt"
	"time"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
)

// Decoder is a RTP/LPCM decoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc3190
type Decoder struct {
	BitDepth     int
	SampleRate   int
	ChannelCount int

	timeDecoder *rtptime.Decoder
	sampleSize  int
}

// Init initializes the decoder.
func (d *Decoder) Init() error {
	d.timeDecoder = rtptime.NewDecoder(d.SampleRate)
	d.sampleSize = d.BitDepth * d.ChannelCount / 8
	return nil
}

// Decode decodes audio samples from a RTP packet.
// It returns audio samples and PTS of the first sample.
func (d *Decoder) Decode(pkt *rtp.Packet) ([]byte, time.Duration, error) {
	plen := len(pkt.Payload)
	if (plen % d.sampleSize) != 0 {
		return nil, 0, fmt.Errorf("received payload of wrong size")
	}

	return pkt.Payload, d.timeDecoder.Decode(pkt.Timestamp), nil
}
