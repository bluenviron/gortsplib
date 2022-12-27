package headers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesJpeg = []struct {
	name string
	enc  []byte
	dec  JPEG
}{
	{
		"base",
		[]byte{
			0x0, 0x0, 0x0, 0x0, 0x1, 0xff, 0x8, 0x4,
		},
		JPEG{
			TypeSpecific: 0,
			Type:         1,
			Quantization: 255,
			Width:        64,
			Height:       32,
		},
	},
}

func TestJpegUnmarshal(t *testing.T) {
	for _, ca := range casesJpeg {
		t.Run(ca.name, func(t *testing.T) {
			var h JPEG
			_, err := h.Unmarshal(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.dec, h)
		})
	}
}

func TestJpegMarshal(t *testing.T) {
	for _, ca := range casesJpeg {
		t.Run(ca.name, func(t *testing.T) {
			buf := ca.dec.Marshal(nil)
			require.Equal(t, ca.enc, buf)
		})
	}
}
