package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
)

func TestTrackURL(t *testing.T) {
	for _, ca := range []struct {
		name    string
		sdp     []byte
		baseURL *base.URL
		ur      *base.URL
	}{
		{
			"missing control",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
		},
		{
			"absolute control",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:rtsp://localhost/path/trackID=7"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/trackID=7"),
		},
		{
			"relative control",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/path/trackID=5"),
		},
		{
			"relative control, subpath",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path/"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path/trackID=5"),
		},
		{
			"relative control, url without slash",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/sub/path/trackID=5"),
		},
		{
			"relative control, url with query",
			[]byte("v=0\r\n" +
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
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:trackID=5"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			mustParseURL("rtsp://myuser:mypass@192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=5"),
		},
		{
			"relative control, control is query",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:?ctype=video"),
			mustParseURL("rtsp://192.168.1.99:554/test"),
			mustParseURL("rtsp://192.168.1.99:554/test?ctype=video"),
		},
		{
			"relative control, control is query and no path",
			[]byte("v=0\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000\r\n" +
				"a=control:?ctype=video"),
			mustParseURL("rtsp://192.168.1.99:554/"),
			mustParseURL("rtsp://192.168.1.99:554/?ctype=video"),
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			tracks, err := ReadTracks(ca.sdp)
			require.NoError(t, err)
			ur, err := tracks[0].URL(ca.baseURL)
			require.NoError(t, err)
			require.Equal(t, ca.ur, ur)
		})
	}
}

func TestTrackClockRate(t *testing.T) {
	for _, ca := range []struct {
		name      string
		sdp       []byte
		clockRate int
	}{
		{
			"empty encoding parameters",
			[]byte("v=0\r\n" +
				"o=- 38990265062388 38990265062388 IN IP4 192.168.1.142\r\n" +
				"s=RTSP Session\r\n" +
				"c=IN IP4 192.168.1.142\r\n" +
				"t=0 0\r\n" +
				"a=control:*\r\n" +
				"a=range:npt=0-\r\n" +
				"m=video 0 RTP/AVP 96\r\n" +
				"a=rtpmap:96 H264/90000 \r\n" +
				"a=range:npt=0-\r\n" +
				"a=framerate:0S\r\n" +
				"a=fmtp:96 profile-level-id=64000c; packetization-mode=1; " +
				"sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==\r\n" +
				"a=framerate:25\r\n" +
				"a=control:trackID=3\r\n"),
			90000,
		},
		{
			"static payload type 1",
			[]byte("v=0\r\n" +
				"o=- 38990265062388 38990265062388 IN IP4 192.168.1.142\r\n" +
				"s=RTSP Session\r\n" +
				"c=IN IP4 192.168.1.142\r\n" +
				"t=0 0\r\n" +
				"a=control:*\r\n" +
				"a=range:npt=0-\r\n" +
				"m=audio 0 RTP/AVP 8\r\n" +
				"a=control:trackID=4"),
			8000,
		},
		{
			"static payload type 2",
			[]byte("v=0\r\n" +
				"o=jdoe 2890844526 2890842807 IN IP4 10.47.16.5\r\n" +
				"s=SDP Seminar\r\n" +
				"i=A Seminar on the session description protocol\r\n" +
				"u=http://www.example.com/seminars/sdp.pdf\r\n" +
				"e=j.doe@example.com (Jane Doe)\r\n" +
				"p=+1 617 555-6011\r\n" +
				"c=IN IP4 224.2.17.12/127\r\n" +
				"b=X-YZ:128\r\n" +
				"b=AS:12345\r\n" +
				"t=2873397496 2873404696\r\n" +
				"t=3034423619 3042462419\r\n" +
				"r=604800 3600 0 90000\r\n" +
				"z=2882844526 -3600 2898848070 0\r\n" +
				"k=prompt\r\n" +
				"a=candidate:0 1 UDP 2113667327 203.0.113.1 54400 typ host\r\n" +
				"a=recvonly\r\n" +
				"m=audio 49170 RTP/AVP 0\r\n" +
				"i=Vivamus a posuere nisl\r\n" +
				"c=IN IP4 203.0.113.1\r\n" +
				"b=X-YZ:128\r\n" +
				"k=prompt\r\n" +
				"a=sendrecv\r\n"),
			8000,
		},
		{
			"multiple formats",
			[]byte("v=0\r\n" +
				"o=RTSP 1853326073 627916868 IN IP4 0.0.0.0\r\n" +
				"s=RTSP server\r\n" +
				"c=IN IP4 0.0.0.0\r\n" +
				"t=0 0\r\n" +
				"a=control:*\r\n" +
				"a=etag:1234567890\r\n" +
				"a=range:npt=0-\r\n" +
				"a=control:*\r\n" +
				"m=video 0 RTP/AVP 98 96\r\n" +
				"a=control:trackID=1\r\n" +
				"b=AS:0\r\n" +
				"a=rtpmap:98 H265/90000\r\n" +
				"a=fmtp:98 profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
				"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=\r\n" +
				"m=application 0 RTP/AVP 107\r\n" +
				"a=control:trackID=3\r\n" +
				"a=rtpmap:107 vnd.onvif.metadata/90000"),
			90000,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			tracks, err := ReadTracks(ca.sdp)
			require.NoError(t, err)

			clockRate, err := tracks[0].ClockRate()
			require.NoError(t, err)

			require.Equal(t, clockRate, ca.clockRate)
		})
	}
}
