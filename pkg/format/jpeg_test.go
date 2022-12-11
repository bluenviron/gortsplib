package format

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJPEGAttributes(t *testing.T) {
	format := &JPEG{}
	require.Equal(t, "JPEG", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(26), format.PayloadType())
}

func TestJPEGClone(t *testing.T) {
	format := &JPEG{}

	clone := format.Clone()
	// require.NotSame(t, format, clone)
	require.Equal(t, format, clone)
}

func TestJPEGMediaDescription(t *testing.T) {
	format := &JPEG{}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "JPEG/90000", rtpmap)
	require.Equal(t, "", fmtp)
}
