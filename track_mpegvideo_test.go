package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackMPEGVideoNew(t *testing.T) {
	track := &TrackMPEGVideo{}
	require.Equal(t, "", track.GetControl())
}

func TestTrackMPEGVideoClone(t *testing.T) {
	track := &TrackMPEGVideo{}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackMPEGVideoMediaDescription(t *testing.T) {
	track := &TrackMPEGVideo{}

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "video",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"32"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
