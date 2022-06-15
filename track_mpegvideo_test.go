package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackMpegVideoNew(t *testing.T) {
	track := NewTrackMpegVideo()
	require.Equal(t, "", track.GetControl())
}

func TestTrackMpegVideoClone(t *testing.T) {
	track := NewTrackMpegVideo()

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackMpegVideoMediaDescription(t *testing.T) {
	track := NewTrackMpegVideo()

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
