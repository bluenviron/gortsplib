package gortsplib //nolint:dupl

import (
	psdp "github.com/pion/sdp/v3"
)

// TrackMPV is a MPEG-1 or MPEG-2 video track.
type TrackMPV struct {
	trackBase
}

// NewTrackMPV allocates a TrackMPV.
func NewTrackMPV() *TrackMPV {
	return &TrackMPV{}
}

func newTrackMPVFromMediaDescription(
	control string) (*TrackMPV, error,
) {
	return &TrackMPV{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackMPV) ClockRate() int {
	return 90000
}

func (t *TrackMPV) clone() Track {
	return &TrackMPV{
		trackBase: t.trackBase,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackMPV) MediaDescription() *psdp.MediaDescription {
	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "video",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"32"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}
