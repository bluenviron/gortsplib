package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStreamType(t *testing.T) {
	require.NotEqual(t, "unknown", StreamTypeRTP.String())
	require.Equal(t, "unknown", StreamType(4).String())
}
