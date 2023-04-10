package formats

import (
	"github.com/pion/rtp"

	"github.com/bluenviron/gortsplib/v3/pkg/formats/rtpmjpeg"
)

// MJPEG is a RTP format that uses the Motion-JPEG codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc2435
type MJPEG struct{}

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

func (f *MJPEG) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
	return nil
}

// Marshal implements Format.
func (f *MJPEG) Marshal() (string, map[string]string) {
	return "JPEG/90000", nil
}

// PTSEqualsDTS implements Format.
func (f *MJPEG) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (f *MJPEG) CreateDecoder() *rtpmjpeg.Decoder {
	d := &rtpmjpeg.Decoder{}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (f *MJPEG) CreateEncoder() *rtpmjpeg.Encoder {
	e := &rtpmjpeg.Encoder{}
	e.Init()
	return e
}
