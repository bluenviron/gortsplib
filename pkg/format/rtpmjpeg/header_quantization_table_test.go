package rtpmjpeg

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var casesQuantizationTable = []struct {
	name string
	enc  []byte
	dec  headerQuantizationTable
}{
	{
		"base",
		append([]byte{0x1, 0x0, 0x0, 0x80}, bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 128/4)...),
		headerQuantizationTable{
			MBZ:       1,
			Precision: 0,
			Tables: [][]byte{
				bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 64/4),
				bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 64/4),
			},
		},
	},
}

func TestHeaderQuantizationTableUnmarshal(t *testing.T) {
	for _, ca := range casesQuantizationTable {
		t.Run(ca.name, func(t *testing.T) {
			var h headerQuantizationTable
			_, err := h.unmarshal(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.dec, h)
		})
	}
}

func TestHeaderQuantizationTableUnmarshalErrors(t *testing.T) {
	for _, length := range []int{64, 192, 256} {
		var h headerQuantizationTable
		_, err := h.unmarshal([]byte{0, 0, byte(length >> 8), byte(length)})
		require.ErrorContains(t, err, "table length")
	}

	var h headerQuantizationTable
	_, err := h.unmarshal(append([]byte{0, 0, 0, 128}, bytes.Repeat([]byte{1}, 127)...))
	require.ErrorContains(t, err, "buffer is too short")
}

func TestHeaderQuantizationTableMarshal(t *testing.T) {
	for _, ca := range casesQuantizationTable {
		t.Run(ca.name, func(t *testing.T) {
			buf := ca.dec.marshal(nil)
			require.Equal(t, ca.enc, buf)
		})
	}
}
