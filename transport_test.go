package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransportString(t *testing.T) {
	tr := TransportUDPMulticast
	require.NotEqual(t, "unknown", tr.String())

	tr = Transport(15)
	require.Equal(t, "unknown", tr.String())
}
