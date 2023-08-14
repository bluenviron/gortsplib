package rtpmjpeg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesJpeg = []struct {
	name string
	enc  []byte
	dec  headerJPEG
}{
	{
		"base",
		[]byte{
			0x0, 0x0, 0x0, 0x0, 0x1, 0xff, 0x8, 0x4,
		},
		headerJPEG{
			TypeSpecific: 0,
			Type:         1,
			Quantization: 255,
			Width:        64,
			Height:       32,
		},
	},
}

func TestHeaderJpegUnmarshal(t *testing.T) {
	for _, ca := range casesJpeg {
		t.Run(ca.name, func(t *testing.T) {
			var h headerJPEG
			_, err := h.unmarshal(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.dec, h)
		})
	}
}

func TestHeaderJpegMarshal(t *testing.T) {
	for _, ca := range casesJpeg {
		t.Run(ca.name, func(t *testing.T) {
			buf := ca.dec.marshal(nil)
			require.Equal(t, ca.enc, buf)
		})
	}
}
