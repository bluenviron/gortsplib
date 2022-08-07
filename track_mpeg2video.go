package gortsplib //nolint:dupl

import (
	psdp "github.com/pion/sdp/v3"
)

// TrackMPEG2Video is a MPEG-1 or MPEG-2 video track.
type TrackMPEG2Video struct {
	trackBase
}

func newTrackMPEG2VideoFromMediaDescription(
	control string) (*TrackMPEG2Video, error,
) {
	return &TrackMPEG2Video{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackMPEG2Video) ClockRate() int {
	return 90000
}

func (t *TrackMPEG2Video) clone() Track {
	return &TrackMPEG2Video{
		trackBase: t.trackBase,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackMPEG2Video) MediaDescription() *psdp.MediaDescription {
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
