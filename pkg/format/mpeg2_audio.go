package format

import (
	"github.com/pion/rtp"
)

// MPEG2Audio is a MPEG-1 or MPEG-2 audio format.
type MPEG2Audio struct{}

// String implements Format.
func (t *MPEG2Audio) String() string {
	return "MPEG2-audio"
}

// ClockRate implements Format.
func (t *MPEG2Audio) ClockRate() int {
	return 90000
}

// PayloadType implements Format.
func (t *MPEG2Audio) PayloadType() uint8 {
	return 14
}

func (t *MPEG2Audio) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	return nil
}

// Marshal implements Format.
func (t *MPEG2Audio) Marshal() (string, string) {
	return "", ""
}

// PTSEqualsDTS implements Format.
func (t *MPEG2Audio) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
