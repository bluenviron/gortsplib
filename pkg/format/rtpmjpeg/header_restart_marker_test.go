package rtpmjpeg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesRestartMarker = []struct {
	name string
	enc  []byte
	dec  headerRestartMarker
}{
	{
		"base",
		[]byte{
			0x4, 0xd2, 0xff, 0xff,
		},
		headerRestartMarker{
			Interval: 1234,
			Count:    0xffff,
		},
	},
}

func TestHeaderRestartMarkerUnmarshal(t *testing.T) {
	for _, ca := range casesRestartMarker {
		t.Run(ca.name, func(t *testing.T) {
			var h headerRestartMarker
			_, err := h.unmarshal(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.dec, h)
		})
	}
}

func TestHeaderRestartMarkerMarshal(t *testing.T) {
	for _, ca := range casesRestartMarker {
		t.Run(ca.name, func(t *testing.T) {
			buf := ca.dec.marshal(nil)
			require.Equal(t, ca.enc, buf)
		})
	}
}
