package url

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustParse(s string) *URL {
	u, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestURLParse(t *testing.T) {
	for _, ca := range []struct {
		name string
		enc  string
		u    *URL
	}{
		{
			"ipv6 stateless",
			`rtsp://user:pa%23ss@[fe80::a8f4:3219:f33e:a072%wl0]:8554/prox%23ied`,
			&URL{
				Scheme: "rtsp",
				Host:   "[fe80::a8f4:3219:f33e:a072%wl0]:8554",
				Path:   "/prox#ied",
				User:   url.UserPassword("user", "pa#ss"),
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			u, err := Parse(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.u, u)
		})
	}
}

func TestURLParseErrors(t *testing.T) {
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
			_, err := Parse(ca.enc)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestURLClone(t *testing.T) {
	u := mustParse("rtsp://localhost:8554/test/stream")
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

func TestURLCloneWithoutCredentials(t *testing.T) {
	u := mustParse("rtsp://user:pass@localhost:8554/test/stream")
	u2 := u.CloneWithoutCredentials()
	u.Host = "otherhost"

	require.Equal(t, &URL{
		Scheme: "rtsp",
		Host:   "otherhost",
		Path:   "/test/stream",
		User:   url.UserPassword("user", "pass"),
	}, u)

	require.Equal(t, &URL{
		Scheme: "rtsp",
		Host:   "localhost:8554",
		Path:   "/test/stream",
	}, u2)
}

func TestURLRTSPPathAndQuery(t *testing.T) {
	for _, ca := range []struct {
		u *URL
		b string
	}{
		{
			mustParse("rtsp://localhost:8554/teststream/trackID=1"),
			"teststream/trackID=1",
		},
		{
			mustParse("rtsp://localhost:8554/test/stream/trackID=1"),
			"test/stream/trackID=1",
		},
		{
			mustParse("rtsp://192.168.1.99:554/test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=1"),
			"test?user=tmp&password=BagRep1&channel=1&stream=0.sdp/trackID=1",
		},
		{
			mustParse("rtsp://192.168.1.99:554/te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
			"te!st?user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1",
		},
		{
			mustParse("rtsp://192.168.1.99:554/user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1"),
			"user=tmp&password=BagRep1!&channel=1&stream=0.sdp/trackID=1",
		},
	} {
		b, ok := ca.u.RTSPPathAndQuery()
		require.Equal(t, true, ok)
		require.Equal(t, ca.b, b)
	}
}
