package formats //nolint:dupl

import (
	"fmt"
	"strconv"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpvp9"
)

// VP9 is a RTP format for the VP9 codec.
// Specification: https://datatracker.ietf.org/doc/html/draft-ietf-payload-vp9-16
type VP9 struct {
	PayloadTyp uint8
	MaxFR      *int
	MaxFS      *int
	ProfileID  *int
}

func (f *VP9) unmarshal(payloadType uint8, _ string, _ string, _ string, fmtp map[string]string) error {
	f.PayloadTyp = payloadType

	for key, val := range fmtp {
		switch key {
		case "max-fr":
			n, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid max-fr: %v", val)
			}

			v2 := int(n)
			f.MaxFR = &v2

		case "max-fs":
			n, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid max-fs: %v", val)
			}

			v2 := int(n)
			f.MaxFS = &v2

		case "profile-id":
			n, err := strconv.ParseUint(val, 10, 31)
			if err != nil {
				return fmt.Errorf("invalid profile-id: %v", val)
			}

			v2 := int(n)
			f.ProfileID = &v2
		}
	}

	return nil
}

// Codec implements Format.
func (f *VP9) Codec() string {
	return "VP9"
}

// String implements Format.
//
// Deprecated: replaced by Codec().
func (f *VP9) String() string {
	return f.Codec()
}

// ClockRate implements Format.
func (f *VP9) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *VP9) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *VP9) RTPMap() string {
	return "VP9/90000"
}

// FMTP implements Format.
func (f *VP9) FMTP() map[string]string {
	fmtp := make(map[string]string)

	if f.MaxFR != nil {
		fmtp["max-fr"] = strconv.FormatInt(int64(*f.MaxFR), 10)
	}
	if f.MaxFS != nil {
		fmtp["max-fs"] = strconv.FormatInt(int64(*f.MaxFS), 10)
	}
	if f.ProfileID != nil {
		fmtp["profile-id"] = strconv.FormatInt(int64(*f.ProfileID), 10)
	}

	return fmtp
}

// PTSEqualsDTS implements Format.
func (f *VP9) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
//
// Deprecated: this has been replaced by CreateDecoder2() that can also return an error.
func (f *VP9) CreateDecoder() *rtpvp9.Decoder {
	d, _ := f.CreateDecoder2()
	return d
}

// CreateDecoder2 creates a decoder able to decode the content of the format.
func (f *VP9) CreateDecoder2() (*rtpvp9.Decoder, error) {
	d := &rtpvp9.Decoder{}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
//
// Deprecated: this has been replaced by CreateEncoder2() that can also return an error.
func (f *VP9) CreateEncoder() *rtpvp9.Encoder {
	e, _ := f.CreateEncoder2()
	return e
}

// CreateEncoder2 creates an encoder able to encode the content of the format.
func (f *VP9) CreateEncoder2() (*rtpvp9.Encoder, error) {
	e := &rtpvp9.Encoder{
		PayloadType: f.PayloadTyp,
	}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
