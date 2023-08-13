package base

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

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
}

func TestBodyUnmarshal(t *testing.T) {
	for _, ca := range casesBody {
		t.Run(ca.name, func(t *testing.T) {
			var p body
			err := p.unmarshal(ca.h, bufio.NewReader(bytes.NewReader(ca.byts)))
			require.NoError(t, err)
			require.Equal(t, ca.byts, []byte(p))
		})
	}
}

func TestBodyMarshal(t *testing.T) {
	for _, ca := range casesBody {
		t.Run(ca.name, func(t *testing.T) {
			buf := body(ca.byts).marshal()
			require.Equal(t, ca.byts, buf)
		})
	}
}

func FuzzBodyUnmarshal(f *testing.F) {
	f.Fuzz(func(t *testing.T, a string, b []byte) {
		var p body
		p.unmarshal( //nolint:errcheck
			Header{
				"Content-Length": HeaderValue{a},
			},
			bufio.NewReader(bytes.NewReader(b)))
	})
}
