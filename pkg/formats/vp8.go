package formats

import (
	"fmt"
	"strconv"

	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpvp8"
)

// VP8 is a RTP format for the VP8 codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc7741
type VP8 struct {
	PayloadTyp uint8
	MaxFR      *int
	MaxFS      *int
}

func (f *VP8) unmarshal(payloadType uint8, _ string, _ string, _ string, fmtp map[string]string) error {
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
		}
	}

	return nil
}

// Codec implements Format.
func (f *VP8) Codec() string {
	return "VP8"
}

// String implements Format.
//
// Deprecated: replaced by Codec().
func (f *VP8) String() string {
	return f.Codec()
}

// ClockRate implements Format.
func (f *VP8) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *VP8) PayloadType() uint8 {
	return f.PayloadTyp
}

// RTPMap implements Format.
func (f *VP8) RTPMap() string {
	return "VP8/90000"
}

// FMTP implements Format.
func (f *VP8) FMTP() map[string]string {
	fmtp := make(map[string]string)

	if f.MaxFR != nil {
		fmtp["max-fr"] = strconv.FormatInt(int64(*f.MaxFR), 10)
	}

	if f.MaxFS != nil {
		fmtp["max-fs"] = strconv.FormatInt(int64(*f.MaxFS), 10)
	}

	return fmtp
}

// PTSEqualsDTS implements Format.
func (f *VP8) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
//
// Deprecated: this has been replaced by CreateDecoder2() that can also return an error.
func (f *VP8) CreateDecoder() *rtpvp8.Decoder {
	d, _ := f.CreateDecoder2()
	return d
}

// CreateDecoder2 creates a decoder able to decode the content of the format.
func (f *VP8) CreateDecoder2() (*rtpvp8.Decoder, error) {
	d := &rtpvp8.Decoder{}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
//
// Deprecated: this has been replaced by CreateEncoder2() that can also return an error.
func (f *VP8) CreateEncoder() *rtpvp8.Encoder {
	e, _ := f.CreateEncoder2()
	return e
}

// CreateEncoder2 creates an encoder able to encode the content of the format.
func (f *VP8) CreateEncoder2() (*rtpvp8.Encoder, error) {
	e := &rtpvp8.Encoder{
		PayloadType: f.PayloadTyp,
	}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
