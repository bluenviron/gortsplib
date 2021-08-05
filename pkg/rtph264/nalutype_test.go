package rtph264

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNALUType(t *testing.T) {
	require.NotEqual(t, "unknown", naluType(10).String())
	require.Equal(t, "unknown", naluType(50).String())
}
