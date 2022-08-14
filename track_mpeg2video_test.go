package gortsplib //nolint:dupl

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackMPEG2VideoAttributes(t *testing.T) {
	track := &TrackMPEG2Video{}
	require.Equal(t, 90000, track.ClockRate())
	require.Equal(t, "", track.GetControl())
}

func TestTrackMPEG2VideoClone(t *testing.T) {
	track := &TrackMPEG2Video{}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackMPEG2VideoMediaDescription(t *testing.T) {
	track := &TrackMPEG2Video{}

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
