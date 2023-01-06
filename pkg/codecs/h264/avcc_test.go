package h264

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var casesAVCC = []struct {
	name string
	enc  []byte
	dec  [][]byte
}{
	{
		"single",
		[]byte{
			0x00, 0x00, 0x00, 0x03,
			0xaa, 0xbb, 0xcc,
		},
		[][]byte{
			{0xaa, 0xbb, 0xcc},
		},
	},
	{
		"multiple",
		[]byte{
			0x00, 0x00, 0x00, 0x02,
			0xaa, 0xbb,
			0x00, 0x00, 0x00, 0x02,
			0xcc, 0xdd,
			0x00, 0x00, 0x00, 0x02,
			0xee, 0xff,
		},
		[][]byte{
			{0xaa, 0xbb},
			{0xcc, 0xdd},
			{0xee, 0xff},
		},
	},
}

func TestAVCCUnmarshal(t *testing.T) {
	for _, ca := range casesAVCC {
		t.Run(ca.name, func(t *testing.T) {
			dec, err := AVCCUnmarshal(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.dec, dec)
		})
	}
}

func TestAVCCMarshal(t *testing.T) {
	for _, ca := range casesAVCC {
		t.Run(ca.name, func(t *testing.T) {
			enc, err := AVCCMarshal(ca.dec)
			require.NoError(t, err)
			require.Equal(t, ca.enc, enc)
		})
	}
}

func TestAVCCUnmarshalError(t *testing.T) {
	for _, ca := range []struct {
		name string
		enc  []byte
		err  string
	}{
		{
			"empty",
			[]byte{},
			"invalid length",
		},
		{
			"invalid length",
			[]byte{0x01},
			"invalid length",
		},
		{
			"invalid length",
			[]byte{0x00, 0x00, 0x00, 0x03},
			"invalid length",
		},
		{
			"too many nalus",
			bytes.Repeat([]byte{0x00, 0x00, 0x00, 0x01, 0x0a}, 21),
			"NALU count (21) exceeds maximum allowed (20)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := AVCCUnmarshal(ca.enc)
			require.EqualError(t, err, ca.err)
		})
	}
}
