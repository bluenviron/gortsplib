package formats

import (
	"github.com/pion/rtp"
)

// MPEG2Video is a RTP format that uses a MPEG-1 or MPEG-2 Video codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc2250
type MPEG2Video struct{}

func (f *MPEG2Video) unmarshal(_ uint8, _ string, _ string, _ string, _ map[string]string) error {
	return nil
}

// String implements Format.
func (f *MPEG2Video) String() string {
	return "MPEG2-video"
}

// ClockRate implements Format.
func (f *MPEG2Video) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *MPEG2Video) PayloadType() uint8 {
	return 32
}

// RTPMap implements Format.
func (f *MPEG2Video) RTPMap() string {
	return ""
}

// FMTP implements Format.
func (f *MPEG2Video) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *MPEG2Video) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
