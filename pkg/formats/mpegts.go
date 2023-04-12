package formats

import (
	"github.com/pion/rtp"
)

// MPEGTS is a RTP format that uses MPEG-TS to wrap underlying tracks.
// Specification: https://datatracker.ietf.org/doc/html/rfc2250
type MPEGTS struct{}

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

func (f *MPEGTS) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp map[string]string) error {
	return nil
}

// Marshal implements Format.
func (f *MPEGTS) Marshal() (string, map[string]string) {
	return "MP2T/90000", nil
}

// PTSEqualsDTS implements Format.
func (f *MPEGTS) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
