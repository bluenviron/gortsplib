package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTransportProtocolString(t *testing.T) {
	tr := TransportUDPMulticast
	require.NotEqual(t, "unknown", tr.String())

	tr = TransportProtocol(15)
	require.Equal(t, "unknown", tr.String())
}
