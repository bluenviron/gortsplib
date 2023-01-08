//go:build go1.18
// +build go1.18

package jpeg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesStartOfScan = []struct {
	name string
	enc  []byte
	dec  StartOfScan
}{
	{
		"base",
		[]byte{
			0xff, 0xda, 0x0, 0xc, 0x3, 0x0, 0x0, 0x1,
			0x11, 0x2, 0x11, 0x0, 0x3f, 0x0,
		},
		StartOfScan{},
	},
}

func TestStartOfScanUnmarshal(t *testing.T) {
	for _, ca := range casesStartOfScan {
		t.Run(ca.name, func(t *testing.T) {
			var h StartOfScan
			err := h.Unmarshal(ca.enc[4:])
			require.NoError(t, err)
			require.Equal(t, ca.dec, h)
		})
	}
}

func TestStartOfScanMarshal(t *testing.T) {
	for _, ca := range casesStartOfScan {
		t.Run(ca.name, func(t *testing.T) {
			byts := ca.dec.Marshal(nil)
			require.Equal(t, ca.enc, byts)
		})
	}
}

func FuzzStartOfScanUnmarshal(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		var h StartOfScan
		h.Unmarshal(b)
	})
}
