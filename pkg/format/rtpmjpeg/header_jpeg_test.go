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
	{
		"restart type 64",
		[]byte{0, 0, 0, 0, 64, 255, 8, 4},
		headerJPEG{Type: 64, Quantization: 255, Width: 64, Height: 32},
	},
	{
		"restart type 65",
		[]byte{0, 0, 0, 0, 65, 255, 8, 4},
		headerJPEG{Type: 65, Quantization: 255, Width: 64, Height: 32},
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

func TestHeaderJpegUnmarshalErrors(t *testing.T) {
	// RFC 2435 reserves quantization values 100-127; 0 is invalid and
	// 128-255 signal an inline table. Every reserved/invalid value must be
	// rejected instead of falling through to makeQuantizationTables, where a
	// value in 100-127 yields a negative scale and a corrupt table.
	for _, q := range []uint8{0, 100, 126, 127} {
		var h headerJPEG
		_, err := h.unmarshal([]byte{0, 0, 0, 0, 1, q, 8, 8})
		require.Error(t, err, "quantization %d must be rejected", q)
	}
}

func TestHeaderJpegUnmarshalUnsupportedTypes(t *testing.T) {
	for _, typ := range []uint8{2, 63, 66, 127, 128} {
		var h headerJPEG
		_, err := h.unmarshal([]byte{0, 0, 0, 0, typ, 255, 8, 4})
		require.ErrorContains(t, err, "not supported")
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
