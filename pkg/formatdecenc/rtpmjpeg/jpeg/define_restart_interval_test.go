package jpeg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesDefineRestartInterval = []struct {
	name string
	enc  []byte
	dec  DefineRestartInterval
}{
	{
		"base",
		[]byte{
			0xff, MarkerDefineRestartInterval, 0x00, 0x04, 0xd0, 0xc7,
		},
		DefineRestartInterval{
			Interval: 53447,
		},
	},
}

func TestDefineRestartIntervalUnmarshal(t *testing.T) {
	for _, ca := range casesDefineRestartInterval {
		t.Run(ca.name, func(t *testing.T) {
			var h DefineRestartInterval
			err := h.Unmarshal(ca.enc[4:])
			require.NoError(t, err)
			require.Equal(t, ca.dec, h)
		})
	}
}
