package gortsplib //nolint:dupl

import (
	psdp "github.com/pion/sdp/v3"
)

// TrackMPEG2Audio is a MPEG-1 or MPEG-2 audio track.
type TrackMPEG2Audio struct {
	trackBase
}

func newTrackMPEG2AudioFromMediaDescription(
	control string,
) (*TrackMPEG2Audio, error,
) {
	return &TrackMPEG2Audio{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackMPEG2Audio) ClockRate() int {
	return 90000
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackMPEG2Audio) MediaDescription() *psdp.MediaDescription {
	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"14"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}

func (t *TrackMPEG2Audio) clone() Track {
	return &TrackMPEG2Audio{
		trackBase: t.trackBase,
	}
}
