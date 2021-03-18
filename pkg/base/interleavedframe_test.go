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
		name: "generic",
		enc:  []byte{0x24, 0x6, 0x0, 0x4, 0x1, 0x2, 0x3, 0x4},
		dec: InterleavedFrame{
			TrackID:    3,
			StreamType: StreamTypeRTP,
			Payload:    []byte{0x01, 0x02, 0x03, 0x04},
		},
	},
}

func TestInterleavedFrameRead(t *testing.T) {
	for _, ca := range casesInterleavedFrame {
		t.Run(ca.name, func(t *testing.T) {
			var f InterleavedFrame
			f.Payload = make([]byte, 1024)
			err := f.Read(bufio.NewReader(bytes.NewBuffer(ca.enc)))
			require.NoError(t, err)
			require.Equal(t, ca.dec, f)
		})
	}
}

func TestInterleavedFrameWrite(t *testing.T) {
	for _, ca := range casesInterleavedFrame {
		t.Run(ca.name, func(t *testing.T) {
			var buf bytes.Buffer
			bw := bufio.NewWriter(&buf)
			err := ca.dec.Write(bw)
			require.NoError(t, err)
			bw.Flush()
			require.Equal(t, ca.enc, buf.Bytes())
		})
	}
}
