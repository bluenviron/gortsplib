package base_test

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
)

var casesRequest = []struct {
	name string
	byts []byte
	req  base.Request
}{
	{
		"options",
		[]byte("OPTIONS rtsp://example.com/media.mp4 RTSP/1.0\r\n" +
			"CSeq: 1\r\n" +
			"Proxy-Require: gzipped-messages\r\n" +
			"Require: implicit-play\r\n" +
			"\r\n"),
		base.Request{
			Method: "OPTIONS",
			URL:    mustParseURL("rtsp://example.com/media.mp4"),
			Header: base.Header{
				"CSeq":          base.HeaderValue{"1"},
				"Require":       base.HeaderValue{"implicit-play"},
				"Proxy-Require": base.HeaderValue{"gzipped-messages"},
			},
		},
	},
	{
		"describe",
		[]byte("DESCRIBE rtsp://example.com/media.mp4 RTSP/1.0\r\n" +
			"Accept: application/sdp\r\n" +
			"CSeq: 2\r\n" +
			"\r\n"),
		base.Request{
			Method: "DESCRIBE",
			URL:    mustParseURL("rtsp://example.com/media.mp4"),
			Header: base.Header{
				"Accept": base.HeaderValue{"application/sdp"},
				"CSeq":   base.HeaderValue{"2"},
			},
		},
	},
	{
		"describe with special chars",
		[]byte("DESCRIBE rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp RTSP/1.0\r\n" +
			"Accept: application/sdp\r\n" +
			"CSeq: 3\r\n" +
			"\r\n"),
		base.Request{
			Method: "DESCRIBE",
			URL:    mustParseURL("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			Header: base.Header{
				"Accept": base.HeaderValue{"application/sdp"},
				"CSeq":   base.HeaderValue{"3"},
			},
		},
	},
	{
		"announce",
		[]byte("ANNOUNCE rtsp://example.com/media.mp4 RTSP/1.0\r\n" +
			"CSeq: 7\r\n" +
			"Content-Length: 306\r\n" +
			"Content-Type: application/sdp\r\n" +
			"Date: 23 Jan 1997 15:35:06 GMT\r\n" +
			"Session: 12345678\r\n" +
			"\r\n" +
			"v=0\n" +
			"o=mhandley 2890844526 2890845468 IN IP4 126.16.64.4\n" +
			"s=SDP Seminar\n" +
			"i=A Seminar on the session description protocol\n" +
			"u=http://www.cs.ucl.ac.uk/staff/M.Handley/sdp.03.ps\n" +
			"e=mjh@isi.edu (Mark Handley)\n" +
			"c=IN IP4 224.2.17.12/127\n" +
			"t=2873397496 2873404696\n" +
			"a=recvonly\n" +
			"m=audio 3456 RTP/AVP 0\n" +
			"m=video 2232 RTP/AVP 31\n"),
		base.Request{
			Method: "ANNOUNCE",
			URL:    mustParseURL("rtsp://example.com/media.mp4"),
			Header: base.Header{
				"CSeq":           base.HeaderValue{"7"},
				"Date":           base.HeaderValue{"23 Jan 1997 15:35:06 GMT"},
				"Session":        base.HeaderValue{"12345678"},
				"Content-Type":   base.HeaderValue{"application/sdp"},
				"Content-Length": base.HeaderValue{"306"},
			},
			Body: []byte("v=0\n" +
				"o=mhandley 2890844526 2890845468 IN IP4 126.16.64.4\n" +
				"s=SDP Seminar\n" +
				"i=A Seminar on the session description protocol\n" +
				"u=http://www.cs.ucl.ac.uk/staff/M.Handley/sdp.03.ps\n" +
				"e=mjh@isi.edu (Mark Handley)\n" +
				"c=IN IP4 224.2.17.12/127\n" +
				"t=2873397496 2873404696\n" +
				"a=recvonly\n" +
				"m=audio 3456 RTP/AVP 0\n" +
				"m=video 2232 RTP/AVP 31\n",
			),
		},
	},
	{
		"get_parameter",
		[]byte("GET_PARAMETER rtsp://example.com/media.mp4 RTSP/1.0\r\n" +
			"CSeq: 9\r\n" +
			"Content-Length: 24\r\n" +
			"Content-Type: text/parameters\r\n" +
			"Session: 12345678\r\n" +
			"\r\n" +
			"packets_received\n" +
			"jitter\n"),
		base.Request{
			Method: "GET_PARAMETER",
			URL:    mustParseURL("rtsp://example.com/media.mp4"),
			Header: base.Header{
				"CSeq":           base.HeaderValue{"9"},
				"Content-Type":   base.HeaderValue{"text/parameters"},
				"Session":        base.HeaderValue{"12345678"},
				"Content-Length": base.HeaderValue{"24"},
			},
			Body: []byte("packets_received\n" +
				"jitter\n",
			),
		},
	},
	{
		"server-side announce",
		[]byte("OPTIONS * RTSP/1.0\r\n" +
			"CSeq: 1\r\n" +
			"User-Agent: RDIPCamera\r\n" +
			"\r\n"),
		base.Request{
			Method: "OPTIONS",
			URL:    nil,
			Header: base.Header{
				"CSeq":       base.HeaderValue{"1"},
				"User-Agent": base.HeaderValue{"RDIPCamera"},
			},
		},
	},
}

func TestRequestUnmarshal(t *testing.T) {
	// keep req global to make sure that all its fields are overridden.
	var req base.Request

	for _, ca := range casesRequest {
		t.Run(ca.name, func(t *testing.T) {
			err := req.Unmarshal(bufio.NewReader(bytes.NewBuffer(ca.byts)))
			require.NoError(t, err)
			require.Equal(t, ca.req, req)
		})
	}
}

func TestRequestMarshal(t *testing.T) {
	for _, ca := range casesRequest {
		t.Run(ca.name, func(t *testing.T) {
			buf, err := ca.req.Marshal()
			require.NoError(t, err)
			require.Equal(t, ca.byts, buf)
		})
	}
}

func TestRequestString(t *testing.T) {
	byts := []byte("OPTIONS rtsp://example.com/media.mp4 RTSP/1.0\r\n" +
		"CSeq: 1\r\n" +
		"Content-Length: 7\r\n" +
		"\r\n" +
		"testing")

	var req base.Request
	err := req.Unmarshal(bufio.NewReader(bytes.NewBuffer(byts)))
	require.NoError(t, err)
	require.Equal(t, string(byts), req.String())
}

func FuzzRequestUnmarshal(f *testing.F) {
	for _, ca := range casesRequest {
		f.Add(ca.byts)
	}

	f.Fuzz(func(t *testing.T, b []byte) {
		var req base.Request
		err := req.Unmarshal(bufio.NewReader(bytes.NewBuffer(b)))
		if err != nil {
			return
		}

		_, err = req.Marshal()
		require.NoError(t, err)
	})
}
