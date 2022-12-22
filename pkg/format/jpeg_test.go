package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestJPEGAttributes(t *testing.T) {
	format := &JPEG{}
	require.Equal(t, "JPEG", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(26), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestJPEGMediaDescription(t *testing.T) {
	format := &JPEG{}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "JPEG/90000", rtpmap)
	require.Equal(t, "", fmtp)
}
