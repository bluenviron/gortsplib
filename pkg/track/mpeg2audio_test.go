package track

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMPEG2AudioAttributes(t *testing.T) {
	track := &MPEG2Audio{}
	require.Equal(t, "MPEG2-audio", track.String())
	require.Equal(t, 90000, track.ClockRate())
	require.Equal(t, uint8(14), track.PayloadType())
}

func TestMPEG2AudioClone(t *testing.T) {
	track := &MPEG2Audio{}

	clone := track.Clone()
	// require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestMPEG2AudioMediaDescription(t *testing.T) {
	track := &MPEG2Audio{}

	rtpmap, fmtp := track.Marshal()
	require.Equal(t, "", rtpmap)
	require.Equal(t, "", fmtp)
}
