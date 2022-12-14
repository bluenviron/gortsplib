package h265

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNALUType(t *testing.T) {
	require.NotEqual(t, true, strings.HasPrefix(NALUType(10).String(), "unknown"))
	require.Equal(t, true, strings.HasPrefix(NALUType(60).String(), "unknown"))
}
