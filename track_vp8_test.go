package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackVP8ttributes(t *testing.T) {
	track := &TrackVP8{}
	require.Equal(t, "VP8", track.String())
	require.Equal(t, 90000, track.ClockRate())
}

func TestTrackVP8Clone(t *testing.T) {
	maxFR := 123
	maxFS := 456
	track := &TrackVP8{
		PayloadType: 96,
		MaxFR:       &maxFR,
		MaxFS:       &maxFS,
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackVP8MediaDescription(t *testing.T) {
	maxFR := 123
	maxFS := 456
	track := &TrackVP8{
		PayloadType: 96,
		MaxFR:       &maxFR,
		MaxFS:       &maxFS,
	}

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "VP8/90000", rtpmap)
	require.Equal(t, "max-fr=123;max-fs=456", fmtp)
}
