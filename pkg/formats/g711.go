package formats

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpsimpleaudio"
)

// G711 is a RTP format that uses the G711 codec, encoded with mu-law or A-law.
// Specification: https://datatracker.ietf.org/doc/html/rfc3551
type G711 struct {
	// whether to use mu-law. Otherwise, A-law is used.
	MULaw bool
}

// String implements Format.
func (f *G711) String() string {
	return "G711"
}

// ClockRate implements Format.
func (f *G711) ClockRate() int {
	return 8000
}

// PayloadType implements Format.
func (f *G711) PayloadType() uint8 {
	if f.MULaw {
		return 0
	}
	return 8
}

func (f *G711) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
	f.MULaw = (payloadType == 0)
	return nil
}

// Marshal implements Format.
func (f *G711) Marshal() (string, map[string]string) {
	if f.MULaw {
		return "PCMU/8000", nil
	}
	return "PCMA/8000", nil
}

// PTSEqualsDTS implements Format.
func (f *G711) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *G711) CreateDecoder() *rtpsimpleaudio.Decoder {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: 8000,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *G711) CreateEncoder() *rtpsimpleaudio.Encoder {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: f.PayloadType(),
		SampleRate:  8000,
	}
	e.Init()
	return e
}
