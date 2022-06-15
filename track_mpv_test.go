package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackMPVNew(t *testing.T) {
	track := NewTrackMPV()
	require.Equal(t, "", track.GetControl())
}

func TestTrackMPVClone(t *testing.T) {
	track := NewTrackMPV()

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackMPVMediaDescription(t *testing.T) {
	track := NewTrackMPV()

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
