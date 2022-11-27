package gortsplib //nolint:dupl

import (
	"github.com/pion/rtp"
)

// TrackMPEG2Audio is a MPEG-1 or MPEG-2 audio track.
type TrackMPEG2Audio struct{}

// String returns a description of the track.
func (t *TrackMPEG2Audio) String() string {
	return "MPEG2-audio"
}

// ClockRate returns the clock rate.
func (t *TrackMPEG2Audio) ClockRate() int {
	return 90000
}

// GetPayloadType returns the payload type.
func (t *TrackMPEG2Audio) GetPayloadType() uint8 {
	return 14
}

func (t *TrackMPEG2Audio) unmarshal(payloadType uint8, clock string, codec string, rtpmap string, fmtp string) error {
	return nil
}

func (t *TrackMPEG2Audio) marshal() (string, string) {
	return "", ""
}

func (t *TrackMPEG2Audio) clone() Track {
	return &TrackMPEG2Audio{}
}

func (t *TrackMPEG2Audio) ptsEqualsDTS(*rtp.Packet) bool {
	return true
}
