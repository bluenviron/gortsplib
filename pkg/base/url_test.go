package base

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestURLInvalid(t *testing.T) {
	for _, ca := range []struct {
		name string
		enc  string
	}{
		{
			"with opaque data",
			"rtsp:opaque?query",
		},
		{
			"with fragment",
			"rtsp://localhost:8554/teststream#fragment",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := ParseURL(ca.enc)
			require.Error(t, err)
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
