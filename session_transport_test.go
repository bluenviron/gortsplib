package gortsplib_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	gortsplib "github.com/bluenviron/gortsplib/v5"
)

func TestProtocolString(t *testing.T) {
	tr := gortsplib.ProtocolUDPMulticast
	require.NotEqual(t, "unknown", tr.String())

	tr = gortsplib.Protocol(15)
	require.Equal(t, "unknown", tr.String())
}
