package gortsplib //nolint:dupl

import (
	"github.com/pion/rtp"
)

// TrackJPEG is a JPEG track.
type TrackJPEG struct{}

// String returns a description of the track.
func (t *TrackJPEG) String() string {
	return "JPEG"
}

// ClockRate returns the clock rate.
func (t *TrackJPEG) ClockRate() int {
	return 90000
}

// GetPayloadType returns the payload type.
func (t *TrackJPEG) GetPayloadType() uint8 {
	return 26
}

func (t *TrackJPEG) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	return nil
}

func (t *TrackJPEG) marshal() (string, string) {
	return "JPEG/90000", ""
}

func (t *TrackJPEG) clone() Track {
	return &TrackJPEG{}
}

func (t *TrackJPEG) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}
