package formats

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestMPEG2VideoAttributes(t *testing.T) {
	format := &MPEG2Video{}
	require.Equal(t, "MPEG2-video", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
