package description

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/sdp"
)

func mustParseURL(s string) *base.URL {
	u, err := base.ParseURL(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestMediaURL(t *testing.T) {
	for _, ca := range []struct {
		name    string
		sdp     []byte
		baseURL *base.URL
		ur      *base.URL
	}{
		{
			"missing control",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
		},
		{
			"absolute control",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:rtsp://localhost/path/trackID=7"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/trackID=7"),
		},
		{
			"absolute control rtsps",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:rtsps://localhost/path/trackID=7"),
			mustParseURL("rtsps://myuser:mypass@192.168.1.99:554/"),
			mustParseURL("rtsps://myuser:mypass@192.168.1.99:554/path/trackID=7"),
		},
		{
			"relative control",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/trackID=5"),
		},
		{
			"relative control, subpath",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path/trackID=5"),
		},
		{
			"relative control, subpath, without slash",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path/trackID=5"),
		},
		{
			"relative control, url with query",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/" +
				"test?user=tmp&password=BagRep1&channel=1&stream=0.sdp"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/" +
				"test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=5"),
		},
		{
			"relative control, url with special chars and query",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/" +
				"te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/" +
				"te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=5"),
		},
		{
			"relative control, url with query without question mark",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=5"),
		},
		{
			"relative control, control is query",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:?ctype=video"),
			mustParseURL("rtsp://192.168.1.99:554/test"),
			mustParseURL("rtsp://192.168.1.99:554/test?ctype=video"),
		},
		{
			"relative control, control is query and no path",
			[]byte("v=0\r\n" +
				"s= \r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:?ctype=video"),
			mustParseURL("rtsp://192.168.1.99:554/"),
			mustParseURL("rtsp://192.168.1.99:554/?ctype=video"),
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var sd sdp.SessionDescription
			err := sd.Unmarshal(ca.sdp)
			require.NoError(t, err)

			var media Media
			err = media.Unmarshal(sd.MediaDescriptions[0])
			require.NoError(t, err)

			ur, err := media.URL(ca.baseURL)
			require.NoError(t, err)
			require.Equal(t, ca.ur, ur)
		})
	}
}

func TestMediaURLError(t *testing.T) {
	media := &Media{
		Type:    "video",
		Formats: []format.Format{&format.H264{}},
	}
	_, err := media.URL(nil)
	require.EqualError(t, err, "Content-Base header not provided")
}
