package gortsplib //nolint:dupl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackJPEGAttributes(t *testing.T) {
	track := &TrackJPEG{}
	require.Equal(t, "JPEG", track.String())
	require.Equal(t, 90000, track.ClockRate())
}

func TestTrackJPEGClone(t *testing.T) {
	track := &TrackJPEG{}

	clone := track.clone()
	// require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackJPEGMediaDescription(t *testing.T) {
	track := &TrackJPEG{}

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "JPEG/90000", rtpmap)
	require.Equal(t, "", fmtp)
}
