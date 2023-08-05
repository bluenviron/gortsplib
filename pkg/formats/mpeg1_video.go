package formats //nolint:dupl

import (
	"github.com/pion/rtp"
)

// MPEG1Video is a RTP format for a MPEG-1/2 Video codec.
// Specification: https://datatracker.ietf.org/doc/html/rfc2250
type MPEG1Video struct{}

func (f *MPEG1Video) unmarshal(_ uint8, _ string, _ string, _ string, _ map[string]string) error {
	return nil
}

// Codec implements Format.
func (f *MPEG1Video) Codec() string {
	return "MPEG-1/2 Video"
}

// String implements Format.
//
// Deprecated: replaced by Codec().
func (f *MPEG1Video) String() string {
	return f.Codec()
}

// ClockRate implements Format.
func (f *MPEG1Video) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *MPEG1Video) PayloadType() uint8 {
	return 32
}

// RTPMap implements Format.
func (f *MPEG1Video) RTPMap() string {
	return ""
}

// FMTP implements Format.
func (f *MPEG1Video) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *MPEG1Video) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}

// MPEG2Video is an alias for MPEG1Video.
//
// Deprecated: replaced by MPEG1Video.
type MPEG2Video = MPEG1Video
