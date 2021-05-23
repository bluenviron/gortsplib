package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestURLError(t *testing.T) {
	for _, ca := range []struct {
		name string
		enc  string
		err  string
	}{
		{
			"invalid",
			":testing",
			"parse \":testing\": missing protocol scheme",
		},
		{
			"unsupported scheme",
			"http://testing",
			"unsupported scheme 'http'",
		},
		{
			"with opaque data",
			"rtsp:opaque?query",
			"URLs with opaque data are not supported",
		},
		{
			"with fragment",
			"rtsp://localhost:8554/teststream#fragment",
			"URLs with fragments are not supported",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := ParseURL(ca.enc)
			require.Equal(t, ca.err, err.Error())
		})
	}
}

func TestURLRTSPPath(t *testing.T) {
	for _, ca := range []struct {
		u *URL
		b string
	}{
		{
			MustParseURL("rtsp://localhost:8554/teststream"),
			"teststream",
		},
		{
			MustParseURL("rtsp://localhost:8554/test/stream"),
			"test/stream",
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp"),
			"test",
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			"te!st",
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			"user=tmp&password=BagRep1!&channel=1&stream=0.sdp",
		},
		{
			MustParseURL("rtsp://localhost:8554/teststream?query1?query2"),
			"teststream",
		},
	} {
		b, ok := ca.u.RTSPPath()
		require.Equal(t, true, ok)
		require.Equal(t, ca.b, b)
	}
}

func TestURLClone(t *testing.T) {
	u := MustParseURL("rtsp://localhost:8554/test/stream")
	u2 := u.Clone()
	u.Host = "otherhost"

	require.Equal(t, &URL{
		Scheme: "rtsp",
		Host:   "otherhost",
		Path:   "/test/stream",
	}, u)

	require.Equal(t, &URL{
		Scheme: "rtsp",
		Host:   "localhost:8554",
		Path:   "/test/stream",
	}, u2)
}

func TestURLErrorRTSPPath(t *testing.T) {
	u := MustParseURL("rtsp://localhost:8554")
	_, ok := u.RTSPPath()
	require.Equal(t, false, ok)
}

func TestURLRTSPPathAndQuery(t *testing.T) {
	for _, ca := range []struct {
		u *URL
		b string
	}{
		{
			MustParseURL("rtsp://localhost:8554/teststream/trackID=1"),
			"teststream/trackID=1",
		},
		{
			MustParseURL("rtsp://localhost:8554/test/stream/trackID=1"),
			"test/stream/trackID=1",
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=1"),
			"test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=1",
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
			"te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1",
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
			"user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1",
		},
	} {
		b, ok := ca.u.RTSPPathAndQuery()
		require.Equal(t, true, ok)
		require.Equal(t, ca.b, b)
	}
}
