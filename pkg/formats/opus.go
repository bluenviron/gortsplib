package formats

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpsimpleaudio"
)

// Opus is a RTP format that uses the Opus codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc7587
type Opus struct {
	PayloadTyp uint8
	IsStereo   bool
}

func (f *Opus) unmarshal(payloadType uint8, clock string, _ string, _ string, fmtp map[string]string) error {
	f.PayloadTyp = payloadType

	tmp := strings.SplitN(clock, "/", 2)
	if len(tmp) != 2 {
		return fmt.Errorf("invalid clock (%v)", clock)
	}

	sampleRate, err := strconv.ParseUint(tmp[0], 10, 31)
	if err != nil || sampleRate != 48000 {
		return fmt.Errorf("invalid sample rate: %d", sampleRate)
	}

	channelCount, err := strconv.ParseUint(tmp[1], 10, 31)
	if err != nil || channelCount != 2 {
		return fmt.Errorf("invalid channel count: %d", channelCount)
	}

	for key, val := range fmtp {
		if key == "sprop-stereo" {
			f.IsStereo = (val == "1")
		}
	}

	return nil
}

// String implements Format.
func (f *Opus) String() string {
	return "Opus"
}

// ClockRate implements Format.
func (f *Opus) ClockRate() int {
	// RFC7587: the RTP timestamp is incremented with a 48000 Hz
	// clock rate for all modes of Opus and all sampling rates.
	return 48000
}

// PayloadType implements Format.
func (f *Opus) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *Opus) RTPMap() string {
	// RFC7587: The RTP clock rate in "a=rtpmap" MUST be 48000, and the
	// number of channels MUST be 2.
	return "opus/48000/2"
}

// FMTP implements Format.
func (f *Opus) FMTP() map[string]string {
	fmtp := map[string]string{
		"sprop-stereo": func() string {
			if f.IsStereo {
				return "1"
			}
			return "0"
		}(),
	}
	return fmtp
}

// PTSEqualsDTS implements Format.
func (f *Opus) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
//
// Deprecated: this has been replaced by CreateDecoder2() that can also return an error.
func (f *Opus) CreateDecoder() *rtpsimpleaudio.Decoder {
	d, _ := f.CreateDecoder2()
	return d
}

// CreateDecoder2 creates a decoder able to decode the content of the format.
func (f *Opus) CreateDecoder2() (*rtpsimpleaudio.Decoder, error) {
	d := &rtpsimpleaudio.Decoder{
		SampleRate: 48000,
	}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
//
// Deprecated: this has been replaced by CreateEncoder2() that can also return an error.
func (f *Opus) CreateEncoder() *rtpsimpleaudio.Encoder {
	e, _ := f.CreateEncoder2()
	return e
}

// CreateEncoder2 creates an encoder able to encode the content of the format.
func (f *Opus) CreateEncoder2() (*rtpsimpleaudio.Encoder, error) {
	e := &rtpsimpleaudio.Encoder{
		PayloadType: f.PayloadTyp,
		SampleRate:  48000,
	}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
