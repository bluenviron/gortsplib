package base

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var cases = []struct {
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
		[]byte("WWW-Authenticate: Digest realm=\"4419b63f5e51\", " +
			"nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"\r\n" +
			"WWW-Authenticate: Basic realm=\"4419b63f5e51\"\r\n" +
			"\r\n"),
		[]byte("WWW-Authenticate: Digest realm=\"4419b63f5e51\", " +
			"nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"\r\n" +
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
		"various",
		[]byte(
			"Testing:\r\n" +
				"content-type: testing\r\n" +
				"content-length:value\r\n" +
				"www-authenticate: value\r\n" +
				"cseq:  value\r\n" +
				"rtp-info: value\r\n" +
				"\r\n"),
		[]byte("CSeq: value\r\n" +
			"Content-Length: value\r\n" +
			"Content-Type: testing\r\n" +
			"Testing: \r\n" +
			"RTP-Info: value\r\n" +
			"WWW-Authenticate: value\r\n" +
			"\r\n"),
		Header{
			"Content-Length":   HeaderValue{"value"},
			"Content-Type":     HeaderValue{"testing"},
			"CSeq":             HeaderValue{"value"},
			"Testing":          HeaderValue{""},
			"RTP-Info":         HeaderValue{"value"},
			"WWW-Authenticate": HeaderValue{"value"},
		},
	},
}

func TestHeaderUnmarshal(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			h := make(Header)
			err := h.unmarshal(bufio.NewReader(bytes.NewBuffer(ca.dec)))
			require.NoError(t, err)
			require.Equal(t, ca.header, h)
		})
	}
}

func TestHeaderWrite(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			buf := ca.header.marshal()
			require.Equal(t, ca.enc, buf)
		})
	}
}

func FuzzHeaderUnmarshal(f *testing.F) {
	str := ""
	for i := 0; i < 300; i++ {
		str += "Key: val\r\n"
	}
	f.Add([]byte(str))

	f.Fuzz(func(t *testing.T, b []byte) {
		var h Header
		h.unmarshal(bufio.NewReader(bytes.NewBuffer(b))) //nolint:errcheck
	})
}
