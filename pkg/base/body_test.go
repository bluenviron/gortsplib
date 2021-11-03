package base

import (
	"bufio"
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

type limitedBuffer struct {
	cap int
	n   int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	b.n += len(p)
	if b.n > b.cap {
		return 0, fmt.Errorf("capacity reached")
	}
	return len(p), nil
}

var casesBody = []struct {
	name string
	h    Header
	byts []byte
}{
	{
		"standard",
		Header{
			"Content-Length": HeaderValue{"4"},
		},
		[]byte{0x01, 0x02, 0x03, 0x04},
	},
	{
		"nil",
		Header{},
		[]byte(nil),
	},
}

func TestBodyRead(t *testing.T) {
	for _, ca := range casesBody {
		t.Run(ca.name, func(t *testing.T) {
			var p body
			err := p.read(ca.h, bufio.NewReader(bytes.NewReader(ca.byts)))
			require.NoError(t, err)
			require.Equal(t, ca.byts, []byte(p))
		})
	}
}

func TestBodyWrite(t *testing.T) {
	for _, ca := range casesBody {
		t.Run(ca.name, func(t *testing.T) {
			var buf bytes.Buffer
			bw := bufio.NewWriter(&buf)
			err := body(ca.byts).write(bw)
			require.NoError(t, err)
			bw.Flush()
			require.Equal(t, ca.byts, buf.Bytes())
		})
	}
}

func TestBodyReadErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		h    Header
		byts []byte
		err  string
	}{
		{
			"invalid body",
			Header{
				"Content-Length": HeaderValue{"17"},
			},
			[]byte("123"),
			"unexpected EOF",
		},
		{
			"invalid content-length",
			Header{
				"Content-Length": HeaderValue{"aaa"},
			},
			[]byte("123"),
			"invalid Content-Length",
		},
		{
			"too big content-length",
			Header{
				"Content-Length": HeaderValue{"1000000"},
			},
			[]byte("123"),
			"Content-Length exceeds 131072 (it's 1000000)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var p body
			err := p.read(ca.h, bufio.NewReader(bytes.NewReader(ca.byts)))
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestBodyWriteErrors(t *testing.T) {
	bw := bufio.NewWriterSize(&limitedBuffer{cap: 3}, 1)
	err := body([]byte("1234")).write(bw)
	require.EqualError(t, err, "capacity reached")
}
