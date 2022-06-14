package gortsplib //nolint:dupl

import (
	psdp "github.com/pion/sdp/v3"
)

// TrackJPEG is a JPEG track.
type TrackJPEG struct {
	trackBase
}

// NewTrackJPEG allocates a TrackJPEG.
func NewTrackJPEG() *TrackJPEG {
	return &TrackJPEG{}
}

func newTrackJPEGFromMediaDescription(
	control string) (*TrackJPEG, error,
) {
	return &TrackJPEG{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackJPEG) ClockRate() int {
	return 90000
}

func (t *TrackJPEG) clone() Track {
	return &TrackJPEG{
		trackBase: t.trackBase,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackJPEG) MediaDescription() *psdp.MediaDescription {
	return &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "video",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"26"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "26 JPEG/90000",
			},
			{
				Key:   "control",
				Value: t.control,
			},
		},
	}
}
