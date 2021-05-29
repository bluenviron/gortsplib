package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefs(t *testing.T) {
	require.NotEqual(t, "unknown", StreamProtocolUDP.String())
	require.Equal(t, "unknown", StreamProtocol(4).String())

	require.NotEqual(t, "unknown", StreamDeliveryUnicast.String())
	require.Equal(t, "unknown", StreamDelivery(4).String())

	require.NotEqual(t, "unknown", StreamTypeRTP.String())
	require.Equal(t, "unknown", StreamType(4).String())
}
