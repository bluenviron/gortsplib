package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTracVP9Clone(t *testing.T) {
	maxFR := 123
	maxFS := 456
	profileID := 789
	track := &TrackVP9{
		PayloadType: 96,
		MaxFR:       &maxFR,
		MaxFS:       &maxFS,
		ProfileID:   &profileID,
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackVP9MediaDescription(t *testing.T) {
	maxFR := 123
	maxFS := 456
	profileID := 789
	track := &TrackVP9{
		PayloadType: 96,
		MaxFR:       &maxFR,
		MaxFS:       &maxFS,
		ProfileID:   &profileID,
	}

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
