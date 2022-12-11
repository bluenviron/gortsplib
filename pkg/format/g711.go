package format

import (
	"fmt"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/rtpcodecs/rtpsimpleaudio"
)

// G711 is a G711 format, encoded with mu-law or A-law.
type G711 struct {
	// whether to use mu-law. Otherwise, A-law is used.
	MULaw bool
}

// String implements Format.
func (t *G711) String() string {
	return "G711"
}

// ClockRate implements Format.
func (t *G711) ClockRate() int {
	return 8000
}

// PayloadType implements Format.
func (t *G711) PayloadType() uint8 {
	if t.MULaw {
		return 0
	}
	return 8
}

func (t *G711) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	tmp := strings.Split(clock, "/")
	if len(tmp) == 2 && tmp[1] != "1" {
		return fmt.Errorf("G711 formats can have only one channel")
	}

	t.MULaw = (payloadType == 0)

	return nil
}

// Marshal implements Format.
func (t *G711) Marshal() (string, string) {
	if t.MULaw {
		return "PCMU/8000", ""
	}
	return "PCMA/8000", ""
}

// Clone implements Format.
func (t *G711) Clone() Format {
	return &G711{
		MULaw: t.MULaw,
	}
}

// PTSEqualsDTS implements Format.
func (t *G711) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (t *G711) CreateDecoder() *rtpsimpleaudio.Decoder {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: 8000,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (t *G711) CreateEncoder() *rtpsimpleaudio.Encoder {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: t.PayloadType(),
		SampleRate:  8000,
	}
	e.Init()
	return e
}
