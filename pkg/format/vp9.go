package format

import (
	"fmt"
	"strconv"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formatdecenc/rtpvp9"
)

// VP9 is a format that uses the VP9 codec.
type VP9 struct {
	PayloadTyp uint8
	MaxFR      *int
	MaxFS      *int
	ProfileID  *int
}

// String implements Format.
func (t *VP9) String() string {
	return "VP9"
}

// ClockRate implements Format.
func (t *VP9) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (t *VP9) PayloadType() uint8 {
	return t.PayloadTyp
}

func (t *VP9) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
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

		case "profile-id":
			n, err := strconv.ParseUint(val, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid profile-id (%v)", val)
			}
			v2 := int(n)
			t.ProfileID = &v2
		}
	}

	return nil
}

// Marshal implements Format.
func (t *VP9) Marshal() (string, map[string]string) {
	fmtp := make(map[string]string)
	if t.MaxFR != nil {
		fmtp["max-fr"] = strconv.FormatInt(int64(*t.MaxFR), 10)
	}
	if t.MaxFS != nil {
		fmtp["max-fs"] = strconv.FormatInt(int64(*t.MaxFS), 10)
	}
	if t.ProfileID != nil {
		fmtp["profile-id"] = strconv.FormatInt(int64(*t.ProfileID), 10)
	}

	return "VP9/90000", fmtp
}

// PTSEqualsDTS implements Format.
func (t *VP9) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (t *VP9) CreateDecoder() *rtpvp9.Decoder {
	d := &rtpvp9.Decoder{}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (t *VP9) CreateEncoder() *rtpvp9.Encoder {
	e := &rtpvp9.Encoder{
		PayloadType: t.PayloadTyp,
	}
	e.Init()
	return e
}
