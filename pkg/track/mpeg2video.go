package track

import (
	"github.com/pion/rtp"
)

// MPEG2Video is a MPEG-1 or MPEG-2 video track.
type MPEG2Video struct{}

// String implements Track.
func (t *MPEG2Video) String() string {
	return "MPEG2-video"
}

// ClockRate implements Track.
func (t *MPEG2Video) ClockRate() int {
	return 90000
}

// PayloadType implements Track.
func (t *MPEG2Video) PayloadType() uint8 {
	return 32
}

func (t *MPEG2Video) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	return nil
}

// Marshal implements Track.
func (t *MPEG2Video) Marshal() (string, string) {
	return "", ""
}

// Clone implements Track.
func (t *MPEG2Video) Clone() Track {
	return &MPEG2Video{}
}

// PTSEqualsDTS implements Track.
func (t *MPEG2Video) PTSEqualsDTS(*rtp.Packet) bool {
	return true
}
