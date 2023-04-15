package formats

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpsimpleaudio"
)

// G722 is a RTP format that uses the G722 codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc3551
type G722 struct{}

func (f *G722) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
	return nil
}

// String implements Format.
func (f *G722) String() string {
	return "G722"
}

// ClockRate implements Format.
func (f *G722) ClockRate() int {
	return 8000
}

// PayloadType implements Format.
func (f *G722) PayloadType() uint8 {
	return 9
}

// RTPMap implements Format.
func (f *G722) RTPMap() string {
	return "G722/8000"
}

// FMTP implements Format.
func (f *G722) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *G722) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *G722) CreateDecoder() *rtpsimpleaudio.Decoder {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: 8000,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *G722) CreateEncoder() *rtpsimpleaudio.Encoder {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: 9,
		SampleRate:  8000,
	}
	e.Init()
	return e
}
