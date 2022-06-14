package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackPCMANew(t *testing.T) {
	track := NewTrackPCMA()
	require.Equal(t, "", track.GetControl())
}

func TestTrackPCMAClone(t *testing.T) {
	track := NewTrackPCMA()

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackPCMAMediaDescription(t *testing.T) {
	track := NewTrackPCMA()

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"8"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "8 PCMA/8000",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
