package format //nolint:revive

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestKLVAttributes(t *testing.T) {
	format := &KLV{
		PayloadTyp: 96,
	}
	require.Equal(t, "KLV", format.Codec())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
