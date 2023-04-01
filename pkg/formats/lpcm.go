package formats

import (
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtplpcm"
)

// LPCM is a format that uses the uncompressed, Linear PCM codec.
type LPCM struct {
	PayloadTyp   uint8
	BitDepth     int
	SampleRate   int
	ChannelCount int
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

func (f *LPCM) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
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

	tmp1, err := strconv.ParseInt(tmp[0], 10, 64)
	if err != nil {
		return err
	}
	f.SampleRate = int(tmp1)

	if len(tmp) >= 2 {
		tmp1, err := strconv.ParseInt(tmp[1], 10, 64)
		if err != nil {
			return err
		}
		f.ChannelCount = int(tmp1)
	} else {
		f.ChannelCount = 1
	}

	return nil
}

// Marshal implements Format.
func (f *LPCM) Marshal() (string, map[string]string) {
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
		"/" + strconv.FormatInt(int64(f.ChannelCount), 10), nil
}

// PTSEqualsDTS implements Format.
func (f *LPCM) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *LPCM) CreateDecoder() *rtplpcm.Decoder {
	d := &rtplpcm.Decoder{
		BitDepth:     f.BitDepth,
		SampleRate:   f.SampleRate,
		ChannelCount: f.ChannelCount,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *LPCM) CreateEncoder() *rtplpcm.Encoder {
	e := &rtplpcm.Encoder{
		PayloadType:  f.PayloadTyp,
		BitDepth:     f.BitDepth,
		SampleRate:   f.SampleRate,
		ChannelCount: f.ChannelCount,
	}
	e.Init()
	return e
}
