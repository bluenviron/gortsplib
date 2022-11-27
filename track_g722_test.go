package gortsplib //nolint:dupl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackG722Attributes(t *testing.T) {
	track := &TrackG722{}
	require.Equal(t, "G722", track.String())
	require.Equal(t, 8000, track.ClockRate())
}

func TestTrackG722Clone(t *testing.T) {
	track := &TrackG722{}

	clone := track.clone()
	// require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackG722MediaDescription(t *testing.T) {
	track := &TrackG722{}

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "G722/8000", rtpmap)
	require.Equal(t, "", fmtp)
}
