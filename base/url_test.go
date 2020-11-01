package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestURLBasePath(t *testing.T) {
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
	} {
		b, ok := ca.u.BasePath()
		require.Equal(t, true, ok)
		require.Equal(t, ca.b, b)
	}
}

func TestURLBaseControlPath(t *testing.T) {
	for _, ca := range []struct {
		u *URL
		b string
		c string
	}{
		{
			MustParseURL("rtsp://localhost:8554/teststream/trackID=1"),
			"teststream",
			"trackID=1",
		},
		{
			MustParseURL("rtsp://localhost:8554/test/stream/trackID=1"),
			"test/stream",
			"trackID=1",
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=1"),
			"test",
			"trackID=1",
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
			"te!st",
			"trackID=1",
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
			"user=tmp&password=BagRep1!&channel=1&stream=0.sdp",
			"trackID=1",
		},
	} {
		b, c, ok := ca.u.BaseControlPath()
		require.Equal(t, true, ok)
		require.Equal(t, ca.b, b)
		require.Equal(t, ca.c, c)
	}
}

func TestURLAddControlPath(t *testing.T) {
	for _, ca := range []struct {
		u  *URL
		ou *URL
	}{
		{
			MustParseURL("rtsp://localhost:8554/teststream"),
			MustParseURL("rtsp://localhost:8554/teststream/trackID=1"),
		},
		{
			MustParseURL("rtsp://localhost:8554/test/stream"),
			MustParseURL("rtsp://localhost:8554/test/stream/trackID=1"),
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp"),
			MustParseURL("rtsp://192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=1"),
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			MustParseURL("rtsp://192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
		},
		{
			MustParseURL("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp"),
			MustParseURL("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
		},
	} {
		ca.u.AddControlPath("trackID=1")
		require.Equal(t, ca.ou, ca.u)
	}
}
