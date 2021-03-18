package base

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
		[]byte("content-type: testing\r\n" +
			"content-length: value\r\n" +
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
		[]byte("www-authenticate: value\r\n" +
			"cseq: value\r\n" +
			"rtp-info: value\r\n" +
			"\r\n"),
		[]byte("CSeq: value\r\n" +
			"RTP-Info: value\r\n" +
			"WWW-Authenticate: value\r\n" +
			"\r\n"),
		Header{
			"CSeq":             HeaderValue{"value"},
			"RTP-Info":         HeaderValue{"value"},
			"WWW-Authenticate": HeaderValue{"value"},
		},
	},
}

func TestHeaderRead(t *testing.T) {
	for _, ca := range casesHeader {
		t.Run(ca.name, func(t *testing.T) {
			h := make(Header)
			err := h.read(bufio.NewReader(bytes.NewBuffer(ca.dec)))
			require.NoError(t, err)
			require.Equal(t, ca.header, h)
		})
	}
}

func TestHeaderWrite(t *testing.T) {
	for _, ca := range casesHeader {
		t.Run(ca.name, func(t *testing.T) {
			var buf bytes.Buffer
			bw := bufio.NewWriter(&buf)
			err := ca.header.write(bw)
			require.NoError(t, err)
			bw.Flush()
			require.Equal(t, ca.enc, buf.Bytes())
		})
	}
}
