package formats

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestMPEGTSAttributes(t *testing.T) {
	format := &MPEGTS{}
	require.Equal(t, "MPEG-TS", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(33), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
