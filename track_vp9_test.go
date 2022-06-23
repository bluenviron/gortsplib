package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackVP9New(t *testing.T) {
	maxFR := 123
	maxFS := 456
	profileID := 789
	track := NewTrackVP9(96, &maxFR, &maxFS, &profileID)
	require.Equal(t, "", track.GetControl())
	require.Equal(t, 123, *track.MaxFR())
	require.Equal(t, 456, *track.MaxFS())
	require.Equal(t, 789, *track.ProfileID())
}

func TestTracVP9Clone(t *testing.T) {
	maxFR := 123
	maxFS := 456
	profileID := 789
	track := NewTrackVP9(96, &maxFR, &maxFS, &profileID)

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackVP9MediaDescription(t *testing.T) {
	maxFR := 123
	maxFS := 456
	profileID := 789
	track := NewTrackVP9(96, &maxFR, &maxFS, &profileID)

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "video",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"96"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "96 VP9/90000",
			},
			{
				Key:   "fmtp",
				Value: "96 max-fr=123;max-fs=456;profile-id=789",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
