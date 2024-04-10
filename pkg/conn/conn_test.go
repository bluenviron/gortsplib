package conn

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
)

func mustParseURL(s string) *base.URL {
	u, err := base.ParseURL(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestRead(t *testing.T) {
	for _, ca := range []struct {
		name string
		enc  []byte
		dec  interface{}
	}{
		{
			"request",
			[]byte("DESCRIBE rtsp://example.com/media.mp4 RTSP/1.0\r\n" +
				"Accept: application/sdp\r\n" +
				"CSeq: 2\r\n" +
				"\r\n"),
			&base.Request{
				Method: base.Describe,
				URL: &base.URL{
					Scheme: "rtsp",
					Host:   "example.com",
					Path:   "/media.mp4",
				},
				Header: base.Header{
					"Accept": base.HeaderValue{"application/sdp"},
					"CSeq":   base.HeaderValue{"2"},
				},
			},
		},
		{
			"response",
			[]byte("RTSP/1.0 200 OK\r\n" +
				"CSeq: 1\r\n" +
				"Public: DESCRIBE, SETUP, TEARDOWN, PLAY, PAUSE\r\n" +
				"\r\n"),
			&base.Response{
				StatusCode:    200,
				StatusMessage: "OK",
				Header: base.Header{
					"CSeq":   base.HeaderValue{"1"},
					"Public": base.HeaderValue{"DESCRIBE, SETUP, TEARDOWN, PLAY, PAUSE"},
				},
			},
		},
		{
			"frame",
			[]byte{0x24, 0x6, 0x0, 0x4, 0x1, 0x2, 0x3, 0x4},
			&base.InterleavedFrame{
				Channel: 6,
				Payload: []byte{0x01, 0x02, 0x03, 0x04},
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			buf := bytes.NewBuffer(ca.enc)
			conn := NewConn(buf)
			dec, err := conn.Read()
			require.NoError(t, err)
			require.Equal(t, ca.dec, dec)
		})
	}
}

func TestReadError(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConn(&buf)
	_, err := conn.Read()
	require.Error(t, err)
}

func TestWriteRequest(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConn(&buf)
	err := conn.WriteRequest(&base.Request{
		Method: "OPTIONS",
		URL:    mustParseURL("rtsp://example.com/media.mp4"),
		Header: base.Header{
			"CSeq":          base.HeaderValue{"1"},
			"Require":       base.HeaderValue{"implicit-play"},
			"Proxy-Require": base.HeaderValue{"gzipped-messages"},
		},
	})
	require.NoError(t, err)
}

func TestWriteResponse(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConn(&buf)
	err := conn.WriteResponse(&base.Response{
		StatusCode:    base.StatusOK,
		StatusMessage: "OK",
		Header: base.Header{
			"CSeq":    base.HeaderValue{"2"},
			"Session": base.HeaderValue{"645252166"},
			"WWW-Authenticate": base.HeaderValue{
				"Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"",
				"Basic realm=\"4419b63f5e51\"",
			},
			"Date": base.HeaderValue{"Sat, Aug 16 2014 02:22:28 GMT"},
		},
	})
	require.NoError(t, err)
}

func TestWriteInterleavedFrame(t *testing.T) {
	var buf bytes.Buffer
	conn := NewConn(&buf)
	err := conn.WriteInterleavedFrame(&base.InterleavedFrame{
		Channel: 6,
		Payload: []byte{0x01, 0x02, 0x03, 0x04},
	}, make([]byte, 1024))
	require.NoError(t, err)
}
