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
	// keep f global to make sure that all its fields are overridden.
	var f InterleavedFrame

	for _, ca := range casesInterleavedFrame {
		t.Run(ca.name, func(t *testing.T) {
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

func TestInterleavedFrameReadErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts []byte
	}{
		{
			"empty",
			[]byte{},
		},
		{
			"invalid magic byte",
			[]byte{0x55, 0x00, 0x00, 0x00},
		},
		{
			"length too big",
			[]byte{0x24, 0x00, 0x00, 0x08},
		},
		{
			"invalid payload",
			[]byte{0x24, 0x00, 0x00, 0x08, 0x01, 0x02},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var f InterleavedFrame
			f.Payload = make([]byte, 5)
			err := f.Read(bufio.NewReader(bytes.NewBuffer(ca.byts)))
			require.Error(t, err)
		})
	}
}
