package h265

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPPSUnmarshal(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts []byte
		pps  PPS
	}{
		{
			"default",
			[]byte{
				0x44, 0x01, 0xc1, 0x72, 0xb4, 0x62, 0x40,
			},
			PPS{},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var pps PPS
			err := pps.Unmarshal(ca.byts)
			require.NoError(t, err)
			require.Equal(t, ca.pps, pps)
		})
	}
}

func FuzzPPSUnmarshal(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		var pps PPS
		pps.Unmarshal(b)
	})
}
