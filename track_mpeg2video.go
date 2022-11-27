package gortsplib //nolint:dupl

import (
	"github.com/pion/rtp"
)

// TrackMPEG2Video is a MPEG-1 or MPEG-2 video track.
type TrackMPEG2Video struct{}

// String returns a description of the track.
func (t *TrackMPEG2Video) String() string {
	return "MPEG2-video"
}

// ClockRate returns the clock rate.
func (t *TrackMPEG2Video) ClockRate() int {
	return 90000
}

// GetPayloadType returns the payload type.
func (t *TrackMPEG2Video) GetPayloadType() uint8 {
	return 32
}

func (t *TrackMPEG2Video) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	return nil
}

func (t *TrackMPEG2Video) marshal() (string, string) {
	return "", ""
}

func (t *TrackMPEG2Video) clone() Track {
	return &TrackMPEG2Video{}
}

func (t *TrackMPEG2Video) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}
