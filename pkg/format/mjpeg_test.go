package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestMJPEGAttributes(t *testing.T) {
	format := &MJPEG{}
	require.Equal(t, "JPEG", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(26), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestMJPEGMediaDescription(t *testing.T) {
	format := &MJPEG{}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "JPEG/90000", rtpmap)
	require.Equal(t, "", fmtp)
}
