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
		"empty",
		[]byte("Testing:\r\n" +
			"\r\n"),
		[]byte("Testing: \r\n" +
			"\r\n"),
		Header{
			"Testing": HeaderValue{""},
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

func TestHeaderReadErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		dec  []byte
		err  string
	}{
		{
			"empty",
			[]byte{},
			"EOF",
		},
		{
			"r without n",
			[]byte("Testing: val\rTesting: val\r\n"),
			"expected '\n', got 'T'",
		},
		{
			"final r without n",
			[]byte("Testing: val\r\nTesting: val\r\n\r"),
			"EOF",
		},
		{
			"missing value",
			[]byte("Testing\r\n"),
			"value is missing",
		},
		{
			"too many entries",
			func() []byte {
				var ret []byte
				for i := 0; i < headerMaxEntryCount+2; i++ {
					ret = append(ret, []byte("Testing: val\r\n")...)
				}
				ret = append(ret, []byte("\r\n")...)
				return ret
			}(),
			"headers count exceeds 255",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			h := make(Header)
			err := h.read(bufio.NewReader(bytes.NewBuffer(ca.dec)))
			require.Equal(t, ca.err, err.Error())
		})
	}
}
