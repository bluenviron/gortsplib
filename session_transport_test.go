package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProtocolString(t *testing.T) {
	tr := ProtocolUDPMulticast
	require.NotEqual(t, "unknown", tr.String())

	tr = Protocol(15)
	require.Equal(t, "unknown", tr.String())
}
