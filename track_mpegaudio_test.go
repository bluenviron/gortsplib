package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackMpegAudioNew(t *testing.T) {
	track := &TrackMpegAudio{}
	require.Equal(t, "", track.GetControl())
}

func TestTrackMpegAudioClone(t *testing.T) {
	track := &TrackMpegAudio{}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackMpegAudioMediaDescription(t *testing.T) {
	track := &TrackMpegAudio{}

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"14"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
