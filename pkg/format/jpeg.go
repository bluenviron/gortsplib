package format

import (
	"github.com/pion/rtp"
)

// JPEG is a JPEG format.
type JPEG struct{}

// String implements Format.
func (t *JPEG) String() string {
	return "JPEG"
}

// ClockRate implements Format.
func (t *JPEG) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (t *JPEG) PayloadType() uint8 {
	return 26
}

func (t *JPEG) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	return nil
}

// Marshal implements Format.
func (t *JPEG) Marshal() (string, string) {
	return "JPEG/90000", ""
}

// PTSEqualsDTS implements Format.
func (t *JPEG) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
