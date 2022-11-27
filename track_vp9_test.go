package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackVP9Attributes(t *testing.T) {
	track := &TrackVP9{}
	require.Equal(t, "VP9", track.String())
	require.Equal(t, 90000, track.ClockRate())
}

func TestTrackVP9Clone(t *testing.T) {
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

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "VP9/90000", rtpmap)
	require.Equal(t, "max-fr=123;max-fs=456;profile-id=789", fmtp)
}
