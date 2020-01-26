package gortsplib

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var casesHeader = []struct {
	name   string
	byts   []byte
	header Header
}{
	{
		"single",
		[]byte("Proxy-Require: gzipped-messages\r\n" +
			"Require: implicit-play\r\n" +
			"\r\n"),
		Header{
			"Require":       []string{"implicit-play"},
			"Proxy-Require": []string{"gzipped-messages"},
		},
	},
	{
		"multiple",
		[]byte("WWW-Authenticate: Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"\r\n" +
			"WWW-Authenticate: Basic realm=\"4419b63f5e51\"\r\n" +
			"\r\n"),
		Header{
			"WWW-Authenticate": []string{
				`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`,
				`Basic realm="4419b63f5e51"`,
			},
		},
	},
}

func TestHeaderRead(t *testing.T) {
	for _, c := range casesHeader {
		t.Run(c.name, func(t *testing.T) {
			req, err := readHeader(bufio.NewReader(bytes.NewBuffer(c.byts)))
			require.NoError(t, err)
			require.Equal(t, c.header, req)
		})
	}
}

func TestHeaderWrite(t *testing.T) {
	for _, c := range casesHeader {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			bw := bufio.NewWriter(&buf)
			err := c.header.write(bw)
			require.NoError(t, err)
			bw.Flush()
			require.Equal(t, c.byts, buf.Bytes())
		})
	}
}
