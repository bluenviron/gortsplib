package gortsplib //nolint:dupl

import (
	psdp "github.com/pion/sdp/v3"
)

// TrackMpegVideo is a MPEG-1 or MPEG-2 video track.
type TrackMpegVideo struct {
	trackBase
}

func newTrackMpegVideoFromMediaDescription(
	control string) (*TrackMpegVideo, error,
) {
	return &TrackMpegVideo{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackMpegVideo) ClockRate() int {
	return 90000
}

func (t *TrackMpegVideo) clone() Track {
	return &TrackMpegVideo{
		trackBase: t.trackBase,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackMpegVideo) MediaDescription() *psdp.MediaDescription {
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
