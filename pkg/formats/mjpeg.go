package formats //nolint:dupl

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmjpeg"
)

// MJPEG is a RTP format that uses the Motion-JPEG codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc2435
type MJPEG struct{}

func (f *MJPEG) unmarshal(_ uint8, _ string, _ string, _ string, _ map[string]string) error {
	return nil
}

// String implements Format.
func (f *MJPEG) String() string {
	return "M-JPEG"
}

// ClockRate implements Format.
func (f *MJPEG) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *MJPEG) PayloadType() uint8 {
	return 26
}

// RTPMap implements Format.
func (f *MJPEG) RTPMap() string {
	return "JPEG/90000"
}

// FMTP implements Format.
func (f *MJPEG) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *MJPEG) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
//
// Deprecated: this has been replaced by CreateDecoder2() that can also return an error.
func (f *MJPEG) CreateDecoder() *rtpmjpeg.Decoder {
	d, _ := f.CreateDecoder2()
	return d
}

// CreateDecoder2 creates a decoder able to decode the content of the format.
func (f *MJPEG) CreateDecoder2() (*rtpmjpeg.Decoder, error) {
	d := &rtpmjpeg.Decoder{}

	err := d.Init()
	if err != nil {
		return nil, err
	}

	return d, nil
}

// CreateEncoder creates an encoder able to encode the content of the format.
//
// Deprecated: this has been replaced by CreateEncoder2() that can also return an error.
func (f *MJPEG) CreateEncoder() *rtpmjpeg.Encoder {
	e, _ := f.CreateEncoder2()
	return e
}

// CreateEncoder2 creates an encoder able to encode the content of the format.
func (f *MJPEG) CreateEncoder2() (*rtpmjpeg.Encoder, error) {
	e := &rtpmjpeg.Encoder{}

	err := e.Init()
	if err != nil {
		return nil, err
	}

	return e, nil
}
