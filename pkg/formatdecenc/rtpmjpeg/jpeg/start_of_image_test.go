package jpeg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStartOfImageMarshal(t *testing.T) {
	buf := StartOfImage{}.Marshal(nil)
	require.Equal(t, []byte{0xff, 0xd8}, buf)
}
