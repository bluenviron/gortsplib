package formats

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestMPEG1VideoAttributes(t *testing.T) {
	format := &MPEG1Video{}
	require.Equal(t, "MPEG-1/2 Video", format.Codec())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
