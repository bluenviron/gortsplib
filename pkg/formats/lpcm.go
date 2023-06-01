package formats

import (
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtplpcm"
)

// LPCM is a RTP format that uses the uncompressed, Linear PCM codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc3190
type LPCM struct {
	PayloadTyp   uint8
	BitDepth     int
	SampleRate   int
	ChannelCount int
}

func (f *LPCM) unmarshal(payloadType uint8, clock string, codec string, _ string, _ map[string]string) error {
	f.PayloadTyp = payloadType

	switch codec {
	case "l8":
		f.BitDepth = 8

	case "l16":
		f.BitDepth = 16

	case "l24":
		f.BitDepth = 24
	}

	tmp := strings.SplitN(clock, "/", 2)

	tmp1, err := strconv.ParseUint(tmp[0], 10, 31)
	if err != nil {
		return err
	}
	f.SampleRate = int(tmp1)

	if len(tmp) >= 2 {
		tmp1, err := strconv.ParseUint(tmp[1], 10, 31)
		if err != nil {
			return err
		}
		f.ChannelCount = int(tmp1)
	} else {
		f.ChannelCount = 1
	}

	return nil
}

// String implements Format.
func (f *LPCM) String() string {
	return "LPCM"
}

// ClockRate implements Format.
func (f *LPCM) ClockRate() int {
	return f.SampleRate
}

// PayloadType implements Format.
func (f *LPCM) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *LPCM) RTPMap() string {
	var codec string
	switch f.BitDepth {
	case 8:
		codec = "L8"

	case 16:
		codec = "L16"

	case 24:
		codec = "L24"
	}

	return codec + "/" + strconv.FormatInt(int64(f.SampleRate), 10) +
		"/" + strconv.FormatInt(int64(f.ChannelCount), 10)
}

// FMTP implements Format.
func (f *LPCM) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *LPCM) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
//
// Deprecated: this has been replaced by CreateDecoder2() that can also return an error.
func (f *LPCM) CreateDecoder() *rtplpcm.Decoder {
	d, _ := f.CreateDecoder2()
	return d
}

// CreateDecoder2 creates a decoder able to decode the content of the format.
func (f *LPCM) CreateDecoder2() (*rtplpcm.Decoder, error) {
	d := &rtplpcm.Decoder{
		BitDepth:     f.BitDepth,
		SampleRate:   f.SampleRate,
		ChannelCount: f.ChannelCount,
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
func (f *LPCM) CreateEncoder() *rtplpcm.Encoder {
	e, _ := f.CreateEncoder2()
	return e
}

// CreateEncoder2 creates an encoder able to encode the content of the format.
func (f *LPCM) CreateEncoder2() (*rtplpcm.Encoder, error) {
	e := &rtplpcm.Encoder{
		PayloadType:  f.PayloadTyp,
		BitDepth:     f.BitDepth,
		SampleRate:   f.SampleRate,
		ChannelCount: f.ChannelCount,
	}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
