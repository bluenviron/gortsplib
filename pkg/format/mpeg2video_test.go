package format

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMPEG2VideoAttributes(t *testing.T) {
	format := &MPEG2Video{}
	require.Equal(t, "MPEG2-video", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(32), format.PayloadType())
}

func TestMPEG2VideoClone(t *testing.T) {
	format := &MPEG2Video{}

	clone := format.Clone()
	// require.NotSame(t, format, clone)
	require.Equal(t, format, clone)
}

func TestMPEG2VideoMediaDescription(t *testing.T) {
	format := &MPEG2Video{}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "", rtpmap)
	require.Equal(t, "", fmtp)
}
