package base

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var casesInterleavedFrame = []struct {
	name string
	enc  []byte
	dec  InterleavedFrame
}{
	{
		name: "rtp",
		enc:  []byte{0x24, 0x6, 0x0, 0x4, 0x1, 0x2, 0x3, 0x4},
		dec: InterleavedFrame{
			Channel: 6,
			Payload: []byte{0x01, 0x02, 0x03, 0x04},
		},
	},
	{
		name: "rtcp",
		enc:  []byte{0x24, 0xd, 0x0, 0x4, 0x5, 0x6, 0x7, 0x8},
		dec: InterleavedFrame{
			Channel: 13,
			Payload: []byte{0x05, 0x06, 0x07, 0x08},
		},
	},
}

func TestInterleavedFrameUnmarshal(t *testing.T) {
	// keep f global to make sure that all its fields are overridden.
	var f InterleavedFrame

	for _, ca := range casesInterleavedFrame {
		t.Run(ca.name, func(t *testing.T) {
			err := f.Unmarshal(bufio.NewReader(bytes.NewBuffer(ca.enc)))
			require.NoError(t, err)
			require.Equal(t, ca.dec, f)
		})
	}
}

func TestInterleavedFrameMarshal(t *testing.T) {
	for _, ca := range casesInterleavedFrame {
		t.Run(ca.name, func(t *testing.T) {
			buf, err := ca.dec.Marshal()
			require.NoError(t, err)
			require.Equal(t, ca.enc, buf)
		})
	}
}

func FuzzInterleavedFrameUnmarshal(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		var f InterleavedFrame
		f.Unmarshal(bufio.NewReader(bytes.NewBuffer(b))) //nolint:errcheck
	})
}
