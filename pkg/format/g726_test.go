package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestG726Attributes(t *testing.T) {
	format := &G726{}
	require.Equal(t, "G726", format.Codec())
	require.Equal(t, 8000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
