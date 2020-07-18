package gortsplib

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var casesHeader = []struct {
	name   string
	dec    []byte
	enc    []byte
	header Header
}{
	{
		"single",
		[]byte("Proxy-Require: gzipped-messages\r\n" +
			"Require: implicit-play\r\n" +
			"\r\n"),
		[]byte("Proxy-Require: gzipped-messages\r\n" +
			"Require: implicit-play\r\n" +
			"\r\n"),
		Header{
			"Require":       HeaderValue{"implicit-play"},
			"Proxy-Require": HeaderValue{"gzipped-messages"},
		},
	},
	{
		"multiple",
		[]byte("WWW-Authenticate: Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"\r\n" +
			"WWW-Authenticate: Basic realm=\"4419b63f5e51\"\r\n" +
			"\r\n"),
		[]byte("WWW-Authenticate: Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"\r\n" +
			"WWW-Authenticate: Basic realm=\"4419b63f5e51\"\r\n" +
			"\r\n"),
		Header{
			"WWW-Authenticate": HeaderValue{
				`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`,
				`Basic realm="4419b63f5e51"`,
			},
		},
	},
	{
		"without space",
		[]byte("CSeq:2\r\n" +
			"\r\n"),
		[]byte("CSeq: 2\r\n" +
			"\r\n"),
		Header{
			"CSeq": HeaderValue{"2"},
		},
	},
	{
		"with multiple spaces",
		[]byte("CSeq:  2\r\n" +
			"\r\n"),
		[]byte("CSeq: 2\r\n" +
			"\r\n"),
		Header{
			"CSeq": HeaderValue{"2"},
		},
	},
	{
		"normalized keys, standard",
		[]byte("Content-type: testing\r\n" +
			"Content-length: value\r\n" +
			"\r\n"),
		[]byte("Content-Length: value\r\n" +
			"Content-Type: testing\r\n" +
			"\r\n"),
		Header{
			"Content-Length": HeaderValue{"value"},
			"Content-Type":   HeaderValue{"testing"},
		},
	},
	{
		"normalized keys, non-standard",
		[]byte("Www-Authenticate: value\r\n" +
			"Cseq: value\r\n" +
			"\r\n"),
		[]byte("CSeq: value\r\n" +
			"WWW-Authenticate: value\r\n" +
			"\r\n"),
		Header{
			"CSeq":             HeaderValue{"value"},
			"WWW-Authenticate": HeaderValue{"value"},
		},
	},
}

func TestHeaderRead(t *testing.T) {
	for _, c := range casesHeader {
		t.Run(c.name, func(t *testing.T) {
			req, err := headerRead(bufio.NewReader(bytes.NewBuffer(c.dec)))
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
			require.Equal(t, c.enc, buf.Bytes())
		})
	}
}
