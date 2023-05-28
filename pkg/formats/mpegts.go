package formats

import (
	"github.com/pion/rtp"
)

// MPEGTS is a RTP format that uses MPEG-TS to wrap underlying tracks.
// Specification: https://datatracker.ietf.org/doc/html/rfc2250
type MPEGTS struct{}

func (f *MPEGTS) unmarshal(_ uint8, _ string, _ string, _ string, _ map[string]string) error {
	return nil
}

// String implements Format.
func (f *MPEGTS) String() string {
	return "MPEG-TS"
}

// ClockRate implements Format.
func (f *MPEGTS) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (f *MPEGTS) PayloadType() uint8 {
	return 33
}

// RTPMap implements Format.
func (f *MPEGTS) RTPMap() string {
	return "MP2T/90000"
}

// FMTP implements Format.
func (f *MPEGTS) FMTP() map[string]string {
	return nil
}

// PTSEqualsDTS implements Format.
func (f *MPEGTS) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
