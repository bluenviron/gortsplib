package track

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestG711Attributes(t *testing.T) {
	track := &G711{}
	require.Equal(t, "G711", track.String())
	require.Equal(t, 8000, track.ClockRate())
}

func TestG711Clone(t *testing.T) {
	track := &G711{}

	clone := track.Clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestG711MediaDescription(t *testing.T) {
	t.Run("pcma", func(t *testing.T) {
		track := &G711{}

		rtpmap, fmtp := track.Marshal()
		require.Equal(t, "PCMA/8000", rtpmap)
		require.Equal(t, "", fmtp)
	})

	t.Run("pcmu", func(t *testing.T) {
		track := &G711{
			MULaw: true,
		}

		rtpmap, fmtp := track.Marshal()
		require.Equal(t, "PCMU/8000", rtpmap)
		require.Equal(t, "", fmtp)
	})
}
