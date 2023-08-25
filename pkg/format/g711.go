package format

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v4/pkg/format/rtpsimpleaudio"
)

// G711 is a RTP format for the G711 codec, encoded with mu-law or A-law.
// Specification: https://datatracker.ietf.org/doc/html/rfc3551
type G711 struct {
	// whether to use mu-law. Otherwise, A-law is used.
	MULaw bool
}

func (f *G711) unmarshal(ctx *unmarshalContext) error {
	f.MULaw = (ctx.payloadType == 0)
	return nil
}

// Codec implements Format.
func (f *G711) Codec() string {
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

// RTPMap implements Format.
func (f *G711) RTPMap() string {
	if f.MULaw {
		return "PCMU/8000"
	}
	return "PCMA/8000"
}

// FMTP implements Format.
func (f *G711) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *G711) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *G711) CreateDecoder() (*rtpsimpleaudio.Decoder, error) {
	d := &rtpsimpleaudio.Decoder{}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *G711) CreateEncoder() (*rtpsimpleaudio.Encoder, error) {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: f.PayloadType(),
	}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
