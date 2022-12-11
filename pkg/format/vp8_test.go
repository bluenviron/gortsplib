package format

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVP8ttributes(t *testing.T) {
	format := &VP8{
		PayloadTyp: 99,
	}
	require.Equal(t, "VP8", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(99), format.PayloadType())
}

func TestVP8Clone(t *testing.T) {
	maxFR := 123
	maxFS := 456
	format := &VP8{
		PayloadTyp: 96,
		MaxFR:      &maxFR,
		MaxFS:      &maxFS,
	}

	clone := format.Clone()
	require.NotSame(t, format, clone)
	require.Equal(t, format, clone)
}

func TestVP8MediaDescription(t *testing.T) {
	maxFR := 123
	maxFS := 456
	format := &VP8{
		PayloadTyp: 96,
		MaxFR:      &maxFR,
		MaxFS:      &maxFS,
	}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "VP8/90000", rtpmap)
	require.Equal(t, "max-fr=123;max-fs=456", fmtp)
}
