package jpeg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesDefineHuffmanTable = []struct {
	name string
	enc  []byte
	dec  DefineHuffmanTable
}{
	{
		"base",
		[]byte{
			0xff, 0xc4, 0x0, 0x7, 0x43, 0x1, 0x2, 0x3, 0x4,
		},
		DefineHuffmanTable{
			Codes:       []byte{0x01, 0x02},
			Symbols:     []byte{0x03, 0x04},
			TableNumber: 3,
			TableClass:  4,
		},
	},
}

func TestDefineHuffmanTableMarshal(t *testing.T) {
	for _, ca := range casesDefineHuffmanTable {
		t.Run(ca.name, func(t *testing.T) {
			byts := ca.dec.Marshal(nil)
			require.Equal(t, ca.enc, byts)
		})
	}
}
