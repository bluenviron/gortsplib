package gortsplib //nolint:dupl

import (
	psdp "github.com/pion/sdp/v3"
)

// TrackMPEGAudio is a MPEG-1 or MPEG-2 audio track.
type TrackMPEGAudio struct {
	trackBase
}

func newTrackMPEGAudioFromMediaDescription(
	control string) (*TrackMPEGAudio, error,
) {
	return &TrackMPEGAudio{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackMPEGAudio) ClockRate() int {
	return 90000
}

func (t *TrackMPEGAudio) clone() Track {
	return &TrackMPEGAudio{
		trackBase: t.trackBase,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackMPEGAudio) MediaDescription() *psdp.MediaDescription {
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
