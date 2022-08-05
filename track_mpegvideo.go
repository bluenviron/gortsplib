package gortsplib //nolint:dupl

import (
	psdp "github.com/pion/sdp/v3"
)

// TrackMPEGVideo is a MPEG-1 or MPEG-2 video track.
type TrackMPEGVideo struct {
	trackBase
}

func newTrackMPEGVideoFromMediaDescription(
	control string) (*TrackMPEGVideo, error,
) {
	return &TrackMPEGVideo{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackMPEGVideo) ClockRate() int {
	return 90000
}

func (t *TrackMPEGVideo) clone() Track {
	return &TrackMPEGVideo{
		trackBase: t.trackBase,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackMPEGVideo) MediaDescription() *psdp.MediaDescription {
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
