package jpeg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesStartOfFrame1 = []struct {
	name string
	enc  []byte
	dec  StartOfFrame1
}{
	{
		"base",
		[]byte{
			0xff, 0xc0, 0x0, 0x11, 0x8, 0x2, 0x58, 0x3,
			0x20, 0x3, 0x0, 0x22, 0x0, 0x1, 0x11, 0x1,
			0x2, 0x11, 0x1,
		},
		StartOfFrame1{
			Type:                   1,
			Width:                  800,
			Height:                 600,
			QuantizationTableCount: 2,
		},
	},
}

func TestStartOfFrame1Unmarshal(t *testing.T) {
	for _, ca := range casesStartOfFrame1 {
		t.Run(ca.name, func(t *testing.T) {
			var h StartOfFrame1
			err := h.Unmarshal(ca.enc[4:])
			require.NoError(t, err)
			h.QuantizationTableCount = 2
			require.Equal(t, ca.dec, h)
		})
	}
}

func TestStartOfFrame1Marshal(t *testing.T) {
	for _, ca := range casesStartOfFrame1 {
		t.Run(ca.name, func(t *testing.T) {
			byts := ca.dec.Marshal(nil)
			require.Equal(t, ca.enc, byts)
		})
	}
}

func FuzzStartOfFrame1Unmarshal(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		var h StartOfFrame1
		h.Unmarshal(b)
	})
}
