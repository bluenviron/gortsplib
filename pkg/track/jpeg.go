package track

import (
	"github.com/pion/rtp"
)

// JPEG is a JPEG track.
type JPEG struct{}

// String implements Track.
func (t *JPEG) String() string {
	return "JPEG"
}

// ClockRate implements Track.
func (t *JPEG) ClockRate() int {
	return 90000
}

// PayloadType implements Track.
func (t *JPEG) PayloadType() uint8 {
	return 26
}

func (t *JPEG) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	return nil
}

// Marshal implements Track.
func (t *JPEG) Marshal() (string, string) {
	return "JPEG/90000", ""
}

// Clone implements Track.
func (t *JPEG) Clone() Track {
	return &JPEG{}
}

// PTSEqualsDTS implements Track.
func (t *JPEG) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
