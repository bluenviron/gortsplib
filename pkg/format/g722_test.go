package format

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestG722Attributes(t *testing.T) {
	format := &G722{}
	require.Equal(t, "G722", format.String())
	require.Equal(t, 8000, format.ClockRate())
	require.Equal(t, uint8(9), format.PayloadType())
}

func TestG722Clone(t *testing.T) {
	format := &G722{}

	clone := format.Clone()
	// require.NotSame(t, format, clone)
	require.Equal(t, format, clone)
}

func TestG722MediaDescription(t *testing.T) {
	format := &G722{}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "G722/8000", rtpmap)
	require.Equal(t, "", fmtp)
}
