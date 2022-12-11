package gortsplib //nolint:dupl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackMPEG2AudioAttributes(t *testing.T) {
	track := &TrackMPEG2Audio{}
	require.Equal(t, "MPEG2-audio", track.String())
	require.Equal(t, 90000, track.ClockRate())
}

func TestTrackMPEG2AudioClone(t *testing.T) {
	track := &TrackMPEG2Audio{}

	clone := track.clone()
	// require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackMPEG2AudioMediaDescription(t *testing.T) {
	track := &TrackMPEG2Audio{}

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "", rtpmap)
	require.Equal(t, "", fmtp)
}
