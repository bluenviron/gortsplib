package rtptime

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncoder(t *testing.T) {
	e := NewEncoder(90000, 12345)

	ts := e.Encode(0)
	require.Equal(t, uint32(12345), ts)
}
