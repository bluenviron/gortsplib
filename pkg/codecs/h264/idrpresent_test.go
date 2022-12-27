package h264

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIDRPresent(t *testing.T) {
	require.Equal(t, true, IDRPresent([][]byte{
		{0x05},
		{0x07},
	}))
	require.Equal(t, false, IDRPresent([][]byte{
		{0x01},
	}))
}
