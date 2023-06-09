package formats //nolint:dupl

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmpeg2audio"
)

// MPEG2Audio is a RTP format for a MPEG-1/2 Audio codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc2250
type MPEG2Audio struct{}

func (f *MPEG2Audio) unmarshal(_ uint8, _ string, _ string, _ string, _ map[string]string) error {
	return nil
}

// Codec implements Format.
func (f *MPEG2Audio) Codec() string {
	return "MPEG-1/2 Audio"
}

// String implements Format.
//
// Deprecated: replaced by Codec().
func (f *MPEG2Audio) String() string {
	return f.Codec()
}

// ClockRate implements Format.
func (f *MPEG2Audio) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *MPEG2Audio) PayloadType() uint8 {
	return 14
}

// RTPMap implements Format.
func (f *MPEG2Audio) RTPMap() string {
	return ""
}

// FMTP implements Format.
func (f *MPEG2Audio) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *MPEG2Audio) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
//
// Deprecated: this has been replaced by CreateDecoder2() that can also return an error.
func (f *MPEG2Audio) CreateDecoder() *rtpmpeg2audio.Decoder {
	d, _ := f.CreateDecoder2()
	return d
}

// CreateDecoder2 creates a decoder able to decode the content of the format.
func (f *MPEG2Audio) CreateDecoder2() (*rtpmpeg2audio.Decoder, error) {
	d := &rtpmpeg2audio.Decoder{}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
//
// Deprecated: this has been replaced by CreateEncoder2() that can also return an error.
func (f *MPEG2Audio) CreateEncoder() *rtpmpeg2audio.Encoder {
	e, _ := f.CreateEncoder2()
	return e
}

// CreateEncoder2 creates an encoder able to encode the content of the format.
func (f *MPEG2Audio) CreateEncoder2() (*rtpmpeg2audio.Encoder, error) {
	e := &rtpmpeg2audio.Encoder{}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
