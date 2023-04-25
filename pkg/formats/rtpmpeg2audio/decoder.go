package rtpmpeg2audio

import (
	"fmt"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg2audio"
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/rtptime"
)

// Decoder is a RTP/MPEG2-audio decoder.
// Specification: https://datatracker.ietf.org/doc/html/rfc2250
type Decoder struct {
	timeDecoder *rtptime.Decoder
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
	if offset != 0 {
		return nil, 0, fmt.Errorf("fragmented units are not supported")
	}

	frames, err := mpeg2audio.SplitFrames(pkt.Payload[4:])
	if err != nil {
		return nil, 0, err
	}

	return frames, d.timeDecoder.Decode(pkt.Timestamp), nil
}
