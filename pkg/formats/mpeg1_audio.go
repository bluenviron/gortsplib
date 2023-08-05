package formats //nolint:dupl

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmpeg1audio"
)

// MPEG1Audio is a RTP format for a MPEG-1/2 Audio codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc2250
type MPEG1Audio struct{}

func (f *MPEG1Audio) unmarshal(_ uint8, _ string, _ string, _ string, _ map[string]string) error {
	return nil
}

// Codec implements Format.
func (f *MPEG1Audio) Codec() string {
	return "MPEG-1/2 Audio"
}

// String implements Format.
//
// Deprecated: replaced by Codec().
func (f *MPEG1Audio) String() string {
	return f.Codec()
}

// ClockRate implements Format.
func (f *MPEG1Audio) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *MPEG1Audio) PayloadType() uint8 {
	return 14
}

// RTPMap implements Format.
func (f *MPEG1Audio) RTPMap() string {
	return ""
}

// FMTP implements Format.
func (f *MPEG1Audio) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *MPEG1Audio) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
//
// Deprecated: this has been replaced by CreateDecoder2() that can also return an error.
func (f *MPEG1Audio) CreateDecoder() *rtpmpeg1audio.Decoder {
	d, _ := f.CreateDecoder2()
	return d
}

// CreateDecoder2 creates a decoder able to decode the content of the format.
func (f *MPEG1Audio) CreateDecoder2() (*rtpmpeg1audio.Decoder, error) {
	d := &rtpmpeg1audio.Decoder{}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
//
// Deprecated: this has been replaced by CreateEncoder2() that can also return an error.
func (f *MPEG1Audio) CreateEncoder() *rtpmpeg1audio.Encoder {
	e, _ := f.CreateEncoder2()
	return e
}

// CreateEncoder2 creates an encoder able to encode the content of the format.
func (f *MPEG1Audio) CreateEncoder2() (*rtpmpeg1audio.Encoder, error) {
	e := &rtpmpeg1audio.Encoder{}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}

// MPEG2Audio is an alias for MPEG1Audio.
//
// Deprecated: replaced by MPEG1Audio.
type MPEG2Audio = MPEG1Audio
