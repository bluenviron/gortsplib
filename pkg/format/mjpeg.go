package format

import (
	"github.com/pion/rtp"

	"github.com/aler9/gortsplib/v2/pkg/formatdecenc/rtpmjpeg"
)

// MJPEG is a format that uses the Motion-JPEG codec.
type MJPEG struct{}

// String implements Format.
func (t *MJPEG) String() string {
	return "M-JPEG"
}

// ClockRate implements Format.
func (t *MJPEG) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (t *MJPEG) PayloadType() uint8 {
	return 26
}

func (t *MJPEG) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
	return nil
}

// Marshal implements Format.
func (t *MJPEG) Marshal() (string, map[string]string) {
	return "JPEG/90000", nil
}

// PTSEqualsDTS implements Format.
func (t *MJPEG) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// CreateDecoder creates a decoder able to decode the content of the format.
func (t *MJPEG) CreateDecoder() *rtpmjpeg.Decoder {
	d := &rtpmjpeg.Decoder{}
	d.Init()
	return d
}

// CreateEncoder creates an encoder able to encode the content of the format.
func (t *MJPEG) CreateEncoder() *rtpmjpeg.Encoder {
	e := &rtpmjpeg.Encoder{}
	e.Init()
	return e
}
