package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackMPEGAudioNew(t *testing.T) {
	track := &TrackMPEGAudio{}
	require.Equal(t, "", track.GetControl())
}

func TestTrackMPEGAudioClone(t *testing.T) {
	track := &TrackMPEGAudio{}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackMPEGAudioMediaDescription(t *testing.T) {
	track := &TrackMPEGAudio{}

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
