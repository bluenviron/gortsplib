package track

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMPEG2VideoAttributes(t *testing.T) {
	track := &MPEG2Video{}
	require.Equal(t, "MPEG2-video", track.String())
	require.Equal(t, 90000, track.ClockRate())
}

func TestMPEG2VideoClone(t *testing.T) {
	track := &MPEG2Video{}

	clone := track.Clone()
	// require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestMPEG2VideoMediaDescription(t *testing.T) {
	track := &MPEG2Video{}

	rtpmap, fmtp := track.Marshal()
	require.Equal(t, "", rtpmap)
	require.Equal(t, "", fmtp)
}
