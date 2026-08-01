package base_test

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
)

func mustParseURL(s string) *base.URL {
	u, err := base.ParseURL(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestParseURL(t *testing.T) {
	for _, ca := range []struct {
		name string
		enc  string
		u    *base.URL
	}{
		{
			"ipv6 stateless",
			`rtsp://user:pa%23ss@[fe80::a8f4:3219:f33e:a072%wl0]:8554/prox%23ied`,
			&base.URL{
				Scheme: "rtsp",
				Host:   "[fe80::a8f4:3219:f33e:a072%wl0]:8554",
				Path:   "/prox#ied",
				User:   url.UserPassword("user", "pa#ss"),
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			u, err := base.ParseURL(ca.enc)
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
			_, err := base.ParseURL(ca.enc)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestURLClone(t *testing.T) {
	u := mustParseURL("rtsp://localhost:8554/test/stream")
	u2 := u.Clone()
	u.Host = "otherhost"

	require.Equal(t, &base.URL{
		Scheme: "rtsp",
		Host:   "otherhost",
		Path:   "/test/stream",
	}, u)

	require.Equal(t, &base.URL{
		Scheme: "rtsp",
		Host:   "localhost:8554",
		Path:   "/test/stream",
	}, u2)
}

func TestURLCloneWithoutCredentials(t *testing.T) {
	u := mustParseURL("rtsp://user:pass@localhost:8554/test/stream")
	u2 := u.CloneWithoutCredentials()
	u.Host = "otherhost"

	require.Equal(t, &base.URL{
		Scheme: "rtsp",
		Host:   "otherhost",
		Path:   "/test/stream",
		User:   url.UserPassword("user", "pass"),
	}, u)

	require.Equal(t, &base.URL{
		Scheme: "rtsp",
		Host:   "localhost:8554",
		Path:   "/test/stream",
	}, u2)
}
