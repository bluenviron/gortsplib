package formats //nolint:dupl

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmpeg2audio"
)

// MPEG2Audio is a RTP format that uses a MPEG-1 or MPEG-2 Audio codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc2250
type MPEG2Audio struct{}

func (f *MPEG2Audio) unmarshal(
	payloadType uint8, clock string, codec string,
	rtpmap string, fmtp map[string]string,
) error {
	return nil
}

// String implements Format.
func (f *MPEG2Audio) String() string {
	return "MPEG2-audio"
}

// ClockRate implements Format.
func (f *MPEG2Audio) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *MPEG2Audio) PayloadType() uint8 {
	return 14
}

// RTPMap implements Format.
func (f *MPEG2Audio) RTPMap() string {
	return ""
}

// FMTP implements Format.
func (f *MPEG2Audio) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *MPEG2Audio) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *MPEG2Audio) CreateDecoder() *rtpmpeg2audio.Decoder {
	d := &rtpmpeg2audio.Decoder{}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *MPEG2Audio) CreateEncoder() *rtpmpeg2audio.Encoder {
	e := &rtpmpeg2audio.Encoder{}
	e.Init()
	return e
}
