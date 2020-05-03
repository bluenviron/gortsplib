package gortsplib

import (
	"bufio"
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

var casesResponse = []struct {
	name string
	byts []byte
	res  *Response
}{
	{
		"ok with single header",
		[]byte("RTSP/1.0 200 OK\r\n" +
			"CSeq: 1\r\n" +
			"Public: DESCRIBE, SETUP, TEARDOWN, PLAY, PAUSE\r\n" +
			"\r\n",
		),
		&Response{
			StatusCode: StatusOK,
			Status:     "OK",
			Header: Header{
				"CSeq":   []string{"1"},
				"Public": []string{"DESCRIBE, SETUP, TEARDOWN, PLAY, PAUSE"},
			},
		},
	},
	{
		"ok with multiple headers",
		[]byte("RTSP/1.0 200 OK\r\n" +
			"CSeq: 2\r\n" +
			"Date: Sat, Aug 16 2014 02:22:28 GMT\r\n" +
			"Session: 645252166\r\n" +
			"WWW-Authenticate: Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"\r\n" +
			"WWW-Authenticate: Basic realm=\"4419b63f5e51\"\r\n" +
			"\r\n",
		),
		&Response{
			StatusCode: StatusOK,
			Status:     "OK",
			Header: Header{
				"CSeq":    []string{"2"},
				"Session": []string{"645252166"},
				"WWW-Authenticate": []string{
					"Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\"",
					"Basic realm=\"4419b63f5e51\"",
				},
				"Date": []string{"Sat, Aug 16 2014 02:22:28 GMT"},
			},
		},
	},
	{
		"ok with content",
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
		&Response{
			StatusCode: 200,
			Status:     "OK",
			Header: Header{
				"Content-Base":   []string{"rtsp://example.com/media.mp4"},
				"Content-Length": []string{"444"},
				"Content-Type":   []string{"application/sdp"},
				"CSeq":           []string{"2"},
			},
			Content: []byte("m=video 0 RTP/AVP 96\n" +
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

func TestResponseRead(t *testing.T) {
	for _, c := range casesResponse {
		t.Run(c.name, func(t *testing.T) {
			res, err := readResponse(bufio.NewReader(bytes.NewBuffer(c.byts)))
			require.NoError(t, err)
			require.Equal(t, c.res, res)
		})
	}
}

func TestResponseWrite(t *testing.T) {
	for _, c := range casesResponse {
		t.Run(c.name, func(t *testing.T) {
			var buf bytes.Buffer
			bw := bufio.NewWriter(&buf)
			err := c.res.write(bw)
			require.NoError(t, err)
			// do NOT call flush(), write() must have already done it
			require.Equal(t, c.byts, buf.Bytes())
		})
	}
}
