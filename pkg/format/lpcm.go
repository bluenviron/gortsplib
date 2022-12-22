package format

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/formatdecenc/rtplpcm"
)

// LPCM is an uncompressed, Linear PCM format.
type LPCM struct {
	PayloadTyp   uint8
	BitDepth     int
	SampleRate   int
	ChannelCount int
}

// String implements Format.
func (t *LPCM) String() string {
	return "LPCM"
}

// ClockRate implements Format.
func (t *LPCM) ClockRate() int {
	return t.SampleRate
}

// PayloadType implements Format.
func (t *LPCM) PayloadType() uint8 {
	return t.PayloadTyp
}

func (t *LPCM) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	t.PayloadTyp = payloadType

	switch codec {
	case "l8":
		t.BitDepth = 8

	case "l16":
		t.BitDepth = 16

	case "l24":
		t.BitDepth = 24
	}

	tmp := strings.SplitN(clock, "/", 2)
	if len(tmp) < 1 {
		return fmt.Errorf("invalid clock (%v)", clock)
	}

	tmp1, err := strconv.ParseInt(tmp[0], 10, 64)
	if err != nil {
		return err
	}
	t.SampleRate = int(tmp1)

	if len(tmp) >= 2 {
		tmp1, err := strconv.ParseInt(tmp[1], 10, 64)
		if err != nil {
			return err
		}
		t.ChannelCount = int(tmp1)
	} else {
		t.ChannelCount = 1
	}

	return nil
}

// Marshal implements Format.
func (t *LPCM) Marshal() (string, string) {
	var codec string
	switch t.BitDepth {
	case 8:
		codec = "L8"

	case 16:
		codec = "L16"

	case 24:
		codec = "L24"
	}

	return codec + "/" + strconv.FormatInt(int64(t.SampleRate), 10) +
		"/" + strconv.FormatInt(int64(t.ChannelCount), 10), ""
}

// PTSEqualsDTS implements Format.
func (t *LPCM) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (t *LPCM) CreateDecoder() *rtplpcm.Decoder {
	d := &rtplpcm.Decoder{
		BitDepth:     t.BitDepth,
		SampleRate:   t.SampleRate,
		ChannelCount: t.ChannelCount,
	}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (t *LPCM) CreateEncoder() *rtplpcm.Encoder {
	e := &rtplpcm.Encoder{
		PayloadType:  t.PayloadTyp,
		BitDepth:     t.BitDepth,
		SampleRate:   t.SampleRate,
		ChannelCount: t.ChannelCount,
	}
	e.Init()
	return e
}
