package headers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesRestartMarker = []struct {
	name string
	enc  []byte
	dec  RestartMarker
}{
	{
		"base",
		[]byte{
			0x4, 0xd2, 0xff, 0xff,
		},
		RestartMarker{
			Interval: 1234,
			Count:    0xffff,
		},
	},
}

func TestRestartMarkerUnmarshal(t *testing.T) {
	for _, ca := range casesRestartMarker {
		t.Run(ca.name, func(t *testing.T) {
			var h RestartMarker
			_, err := h.Unmarshal(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.dec, h)
		})
	}
}

func TestRestartMarkerMarshal(t *testing.T) {
	for _, ca := range casesRestartMarker {
		t.Run(ca.name, func(t *testing.T) {
			buf := ca.dec.Marshal(nil)
			require.Equal(t, ca.enc, buf)
		})
	}
}
