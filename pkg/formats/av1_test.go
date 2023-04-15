package formats //nolint:dupl

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestAV1Attributes(t *testing.T) {
	format := &AV1{
		PayloadTyp: 100,
	}
	require.Equal(t, "AV1", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
