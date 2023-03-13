package format

import (
	"fmt"
	"strconv"

	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/formatdecenc/rtpvp8"
)

// VP8 is a format that uses the VP8 codec.
type VP8 struct {
	PayloadTyp uint8
	MaxFR      *int
	MaxFS      *int
}

// String implements Format.
func (t *VP8) String() string {
	return "VP8"
}

// ClockRate implements Format.
func (t *VP8) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (t *VP8) PayloadType() uint8 {
	return t.PayloadTyp
}

func (t *VP8) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
	t.PayloadTyp = payloadType

	for key, val := range fmtp {
		switch key {
		case "max-fr":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid max-fr (%v)", val)
			}
			v2 := int(n)
			t.MaxFR = &v2

		case "max-fs":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid max-fs (%v)", val)
			}
			v2 := int(n)
			t.MaxFS = &v2
		}
	}

	return nil
}

// Marshal implements Format.
func (t *VP8) Marshal() (string, map[string]string) {
	fmtp := make(map[string]string)
	if t.MaxFR != nil {
		fmtp["max-fr"] = strconv.FormatInt(int64(*t.MaxFR), 10)
	}
	if t.MaxFS != nil {
		fmtp["max-fs"] = strconv.FormatInt(int64(*t.MaxFS), 10)
	}

	return "VP8/90000", fmtp
}

// PTSEqualsDTS implements Format.
func (t *VP8) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (t *VP8) CreateDecoder() *rtpvp8.Decoder {
	d := &rtpvp8.Decoder{}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (t *VP8) CreateEncoder() *rtpvp8.Encoder {
	e := &rtpvp8.Encoder{
		PayloadType: t.PayloadTyp,
	}
	e.Init()
	return e
}
