package base64streamreader

import (
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type dummyReader struct {
	input []string
	pos   int
}

func (r *dummyReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.input) {
		return 0, io.EOF
	}

	n := copy(p, r.input[r.pos])
	r.pos++

	return n, nil
}

func TestReader(t *testing.T) {
	for _, ca := range []struct {
		name   string
		input  []string
		output []string
	}{
		{
			"standard",
			[]string{
				"dGVzdGluZyAxIDIgMw==",
			},
			[]string{"testing 1 2 3"},
		},
		{
			"concatenated",
			[]string{
				"dGVzdGluZyAxIDIgMw==b3RoZXIgdGVzdA==",
			},
			[]string{
				"testing 1 2 3",
				"other test",
			},
		},
		{
			"splitted evenly",
			[]string{
				"dGVz",
				"dGluZyAxIDIgMw==",
			},
			[]string{
				"tes",
				"ting 1 2 3",
			},
		},
		{
			"splitted unevenly",
			[]string{
				"dGV",
				"zdGluZyAxIDIgMw==",
			},
			[]string{
				"testing 1 2 3",
			},
		},
		{
			"concatenated and splitted evenly",
			[]string{
				"dGVzdGluZyAxIDIgMw==b3RoZXIgdGVz",
				"dA==",
			},
			[]string{
				"testing 1 2 3",
				"other tes",
				"t",
			},
		},
		{
			"concatenated and splitted unevenly",
			[]string{
				"dGVzdGluZyAxIDIgMw==b3RoZXIgdGVzdA=",
				"=",
			},
			[]string{
				"testing 1 2 3",
				"other tes",
				"t",
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			dr := &dummyReader{input: ca.input}
			r := New(dr)

			var output []string

			for {
				buf := make([]byte, 512)
				n, err := r.Read(buf)
				if err == io.EOF {
					break
				}
				require.NoError(t, err)

				output = append(output, string(buf[:n]))
			}

			require.Equal(t, ca.output, output)
		})
	}
}
