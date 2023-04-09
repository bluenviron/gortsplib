package base

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var casesResponse = []struct {
	name string
	byts []byte
	res  Response
}{
	{
		"ok",
		[]byte("RTSP/1.0 200 OK\r\n" +
			"CSeq: 2\r\n" +
			"Date: Sat, Aug 16 2014 02:22:28 GMT\r\n" +
			"Session: 645252166\r\n" +
			"WWW-Authenticate: Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"\r\n" +
			"WWW-Authenticate: Basic realm=\"4419b63f5e51\"\r\n" +
			"\r\n",
		),
		Response{
			StatusCode:    StatusOK,
			StatusMessage: "OK",
			Header: Header{
				"CSeq":    HeaderValue{"2"},
				"Session": HeaderValue{"645252166"},
				"WWW-Authenticate": HeaderValue{
					"Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"",
					"Basic realm=\"4419b63f5e51\"",
				},
				"Date": HeaderValue{"Sat, Aug 16 2014 02:22:28 GMT"},
			},
		},
	},
	{
		"ok with payload",
		[]byte("RTSP/1.0 200 OK\r\n" +
			"CSeq: 2\r\n" +
			"Content-Base: rtsp://example.com/media.mp4\r\n" +
			"Content-Length: 444\r\n" +
			"Content-Type: application/sdp\r\n" +
			"\r\n" +
			"m=video 0 RTP/AVP 96\n" +
			"a=control:streamid=0\n" +
			"a=range:npt=0-7.741000\n" +
			"a=length:npt=7.741000\n" +
			"a=rtpmap:96 MP4V-ES/5544\n" +
			"a=mimetype:string;\"video/MP4V-ES\"\n" +
			"a=AvgBitRate:integer;304018\n" +
			"a=StreamName:string;\"hinted video track\"\n" +
			"m=audio 0 RTP/AVP 97\n" +
			"a=control:streamid=1\n" +
			"a=range:npt=0-7.712000\n" +
			"a=length:npt=7.712000\n" +
			"a=rtpmap:97 mpeg4-generic/32000/2\n" +
			"a=mimetype:string;\"audio/mpeg4-generic\"\n" +
			"a=AvgBitRate:integer;65790\n" +
			"a=StreamName:string;\"hinted audio track\"\n",
		),
		Response{
			StatusCode:    200,
			StatusMessage: "OK",
			Header: Header{
				"Content-Base":   HeaderValue{"rtsp://example.com/media.mp4"},
				"Content-Length": HeaderValue{"444"},
				"Content-Type":   HeaderValue{"application/sdp"},
				"CSeq":           HeaderValue{"2"},
			},
			Body: []byte("m=video 0 RTP/AVP 96\n" +
				"a=control:streamid=0\n" +
				"a=range:npt=0-7.741000\n" +
				"a=length:npt=7.741000\n" +
				"a=rtpmap:96 MP4V-ES/5544\n" +
				"a=mimetype:string;\"video/MP4V-ES\"\n" +
				"a=AvgBitRate:integer;304018\n" +
				"a=StreamName:string;\"hinted video track\"\n" +
				"m=audio 0 RTP/AVP 97\n" +
				"a=control:streamid=1\n" +
				"a=range:npt=0-7.712000\n" +
				"a=length:npt=7.712000\n" +
				"a=rtpmap:97 mpeg4-generic/32000/2\n" +
				"a=mimetype:string;\"audio/mpeg4-generic\"\n" +
				"a=AvgBitRate:integer;65790\n" +
				"a=StreamName:string;\"hinted audio track\"\n",
			),
		},
	},
}

func TestResponseUnmarshal(t *testing.T) {
	// keep res global to make sure that all its fields are overridden.
	var res Response

	for _, c := range casesResponse {
		t.Run(c.name, func(t *testing.T) {
			err := res.Unmarshal(bufio.NewReader(bytes.NewBuffer(c.byts)))
			require.NoError(t, err)
			require.Equal(t, c.res, res)
		})
	}
}

func TestResponseMarshal(t *testing.T) {
	for _, c := range casesResponse {
		t.Run(c.name, func(t *testing.T) {
			buf, err := c.res.Marshal()
			require.NoError(t, err)
			require.Equal(t, c.byts, buf)
		})
	}
}

func TestResponseMarshalAutoFillStatus(t *testing.T) {
	res := &Response{
		StatusCode: StatusMethodNotAllowed,
		Header: Header{
			"CSeq":    HeaderValue{"2"},
			"Session": HeaderValue{"645252166"},
			"WWW-Authenticate": HeaderValue{
				"Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"",
				"Basic realm=\"4419b63f5e51\"",
			},
			"Date": HeaderValue{"Sat, Aug 16 2014 02:22:28 GMT"},
		},
	}
	byts := []byte("RTSP/1.0 405 Method Not Allowed\r\n" +
		"CSeq: 2\r\n" +
		"Date: Sat, Aug 16 2014 02:22:28 GMT\r\n" +
		"Session: 645252166\r\n" +
		"WWW-Authenticate: Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"\r\n" +
		"WWW-Authenticate: Basic realm=\"4419b63f5e51\"\r\n" +
		"\r\n",
	)

	buf, err := res.Marshal()
	require.NoError(t, err)
	require.Equal(t, byts, buf)
}

func TestResponseString(t *testing.T) {
	byts := []byte("RTSP/1.0 200 OK\r\n" +
		"CSeq: 3\r\n" +
		"Content-Length: 7\r\n" +
		"\r\n" +
		"testing")

	var res Response
	err := res.Unmarshal(bufio.NewReader(bytes.NewBuffer(byts)))
	require.NoError(t, err)
	require.Equal(t, string(byts), res.String())
}

func FuzzResponseUnmarshal(f *testing.F) {
	f.Add([]byte("RTSP/1.0 "))

	f.Add([]byte("RTSP/1.0 200 OK\r\n" +
		"Content-Length: 100\r\n" +
		"\r\n" +
		"testing"))

	f.Fuzz(func(t *testing.T, b []byte) {
		var res Response
		res.Unmarshal(bufio.NewReader(bytes.NewBuffer(b)))
	})
}
