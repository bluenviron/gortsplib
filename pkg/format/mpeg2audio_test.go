package format

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMPEG2AudioAttributes(t *testing.T) {
	format := &MPEG2Audio{}
	require.Equal(t, "MPEG2-audio", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(14), format.PayloadType())
}

func TestMPEG2AudioClone(t *testing.T) {
	format := &MPEG2Audio{}

	clone := format.Clone()
	// require.NotSame(t, format, clone)
	require.Equal(t, format, clone)
}

func TestMPEG2AudioMediaDescription(t *testing.T) {
	format := &MPEG2Audio{}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "", rtpmap)
	require.Equal(t, "", fmtp)
}
