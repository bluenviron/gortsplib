package track

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVP8ttributes(t *testing.T) {
	track := &VP8{}
	require.Equal(t, "VP8", track.String())
	require.Equal(t, 90000, track.ClockRate())
}

func TestVP8Clone(t *testing.T) {
	maxFR := 123
	maxFS := 456
	track := &VP8{
		PayloadTyp: 96,
		MaxFR:      &maxFR,
		MaxFS:      &maxFS,
	}

	clone := track.Clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestVP8MediaDescription(t *testing.T) {
	maxFR := 123
	maxFS := 456
	track := &VP8{
		PayloadTyp: 96,
		MaxFR:      &maxFR,
		MaxFS:      &maxFS,
	}

	rtpmap, fmtp := track.Marshal()
	require.Equal(t, "VP8/90000", rtpmap)
	require.Equal(t, "max-fr=123;max-fs=456", fmtp)
}
