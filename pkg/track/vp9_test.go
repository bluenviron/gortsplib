package track

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVP9Attributes(t *testing.T) {
	track := &VP9{}
	require.Equal(t, "VP9", track.String())
	require.Equal(t, 90000, track.ClockRate())
}

func TestVP9Clone(t *testing.T) {
	maxFR := 123
	maxFS := 456
	profileID := 789
	track := &VP9{
		PayloadTyp: 96,
		MaxFR:      &maxFR,
		MaxFS:      &maxFS,
		ProfileID:  &profileID,
	}

	clone := track.Clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestVP9MediaDescription(t *testing.T) {
	maxFR := 123
	maxFS := 456
	profileID := 789
	track := &VP9{
		PayloadTyp: 96,
		MaxFR:      &maxFR,
		MaxFS:      &maxFS,
		ProfileID:  &profileID,
	}

	rtpmap, fmtp := track.Marshal()
	require.Equal(t, "VP9/90000", rtpmap)
	require.Equal(t, "max-fr=123;max-fs=456;profile-id=789", fmtp)
}
