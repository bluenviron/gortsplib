package gortsplib //nolint:dupl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackMPEG2VideoAttributes(t *testing.T) {
	track := &TrackMPEG2Video{}
	require.Equal(t, "MPEG2-video", track.String())
	require.Equal(t, 90000, track.ClockRate())
}

func TestTrackMPEG2VideoClone(t *testing.T) {
	track := &TrackMPEG2Video{}

	clone := track.clone()
	// require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackMPEG2VideoMediaDescription(t *testing.T) {
	track := &TrackMPEG2Video{}

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "", rtpmap)
	require.Equal(t, "", fmtp)
}
