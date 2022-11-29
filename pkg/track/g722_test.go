package track

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestG722Attributes(t *testing.T) {
	track := &G722{}
	require.Equal(t, "G722", track.String())
	require.Equal(t, 8000, track.ClockRate())
}

func TestG722Clone(t *testing.T) {
	track := &G722{}

	clone := track.Clone()
	// require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestG722MediaDescription(t *testing.T) {
	track := &G722{}

	rtpmap, fmtp := track.Marshal()
	require.Equal(t, "G722/8000", rtpmap)
	require.Equal(t, "", fmtp)
}
