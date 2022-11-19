package gortsplib //nolint:dupl

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackMPEG2AudioAttributes(t *testing.T) {
	track := &TrackMPEG2Audio{}
	require.Equal(t, "MPEG2-audio", track.String())
	require.Equal(t, 90000, track.ClockRate())
	require.Equal(t, "", track.GetControl())
}

func TestTrackMPEG2AudioClone(t *testing.T) {
	track := &TrackMPEG2Audio{}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackMPEG2AudioMediaDescription(t *testing.T) {
	track := &TrackMPEG2Audio{}

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
