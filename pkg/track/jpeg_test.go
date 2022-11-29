package track

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJPEGAttributes(t *testing.T) {
	track := &JPEG{}
	require.Equal(t, "JPEG", track.String())
	require.Equal(t, 90000, track.ClockRate())
	require.Equal(t, uint8(26), track.PayloadType())
}

func TestJPEGClone(t *testing.T) {
	track := &JPEG{}

	clone := track.Clone()
	// require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestJPEGMediaDescription(t *testing.T) {
	track := &JPEG{}

	rtpmap, fmtp := track.Marshal()
	require.Equal(t, "JPEG/90000", rtpmap)
	require.Equal(t, "", fmtp)
}
