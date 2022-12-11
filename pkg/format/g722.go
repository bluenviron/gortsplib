package format

import (
	"fmt"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/rtpcodecs/rtpsimpleaudio"
)

// G722 is a G722 format.
type G722 struct{}

// String implements Format.
func (t *G722) String() string {
	return "G722"
}

// ClockRate implements Format.
func (t *G722) ClockRate() int {
	return 8000
}

// PayloadType implements Format.
func (t *G722) PayloadType() uint8 {
	return 9
}

func (t *G722) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	tmp := strings.Split(clock, "/")
	if len(tmp) == 2 && tmp[1] != "1" {
		return fmt.Errorf("G722 formats can have only one channel")
	}

	return nil
}

// Marshal implements Format.
func (t *G722) Marshal() (string, string) {
	return "G722/8000", ""
}

// Clone implements Format.
func (t *G722) Clone() Format {
	return &G722{}
}

// PTSEqualsDTS implements Format.
func (t *G722) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (t *G722) CreateDecoder() *rtpsimpleaudio.Decoder {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: 8000,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (t *G722) CreateEncoder() *rtpsimpleaudio.Encoder {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: 9,
		SampleRate:  8000,
	}
	e.Init()
	return e
}
