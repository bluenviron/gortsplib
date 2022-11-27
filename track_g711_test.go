package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackG711Attributes(t *testing.T) {
	track := &TrackG711{}
	require.Equal(t, "G711", track.String())
	require.Equal(t, 8000, track.ClockRate())
}

func TestTrackG711Clone(t *testing.T) {
	track := &TrackG711{}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackG711MediaDescription(t *testing.T) {
	t.Run("pcma", func(t *testing.T) {
		track := &TrackG711{}

		rtpmap, fmtp := track.marshal()
		require.Equal(t, "PCMA/8000", rtpmap)
		require.Equal(t, "", fmtp)
	})

	t.Run("pcmu", func(t *testing.T) {
		track := &TrackG711{
			MULaw: true,
		}

		rtpmap, fmtp := track.marshal()
		require.Equal(t, "PCMU/8000", rtpmap)
		require.Equal(t, "", fmtp)
	})
}
