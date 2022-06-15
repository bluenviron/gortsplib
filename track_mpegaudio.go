package gortsplib //nolint:dupl

import (
	psdp "github.com/pion/sdp/v3"
)

// TrackMpegAudio is a MPEG-1 or MPEG-2 audio track.
type TrackMpegAudio struct {
	trackBase
}

// NewTrackMpegAudio allocates a TrackMpegAudio.
func NewTrackMpegAudio() *TrackMpegAudio {
	return &TrackMpegAudio{}
}

func newTrackMpegAudioFromMediaDescription(
	control string) (*TrackMpegAudio, error,
) {
	return &TrackMpegAudio{
		trackBase: trackBase{
			control: control,
		},
	}, nil
}

// ClockRate returns the track clock rate.
func (t *TrackMpegAudio) ClockRate() int {
	return 90000
}

func (t *TrackMpegAudio) clone() Track {
	return &TrackMpegAudio{
		trackBase: t.trackBase,
	}
}

// MediaDescription returns the track media description in SDP format.
func (t *TrackMpegAudio) MediaDescription() *psdp.MediaDescription {
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
