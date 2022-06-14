package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/url"
)

func mustParseURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

func TestAuth(t *testing.T) {
	for _, c1 := range []struct {
		name    string
		methods []headers.AuthMethod
	}{
		{
			"basic",
			[]headers.AuthMethod{headers.AuthBasic},
		},
		{
			"digest",
			[]headers.AuthMethod{headers.AuthDigest},
		},
		{
			"both",
			nil,
		},
	} {
		for _, conf := range []string{
			"nofail",
			"wronguser",
			"wrongpass",
			"wrongurl",
		} {
			if conf == "wrongurl" && c1.name == "basic" {
				continue
			}

			t.Run(c1.name+"_"+conf, func(t *testing.T) {
				va := NewValidator("testuser", "testpass", c1.methods)
				wwwAuthenticate := va.Header()

				se, err := NewSender(wwwAuthenticate,
					func() string {
						if conf == "wronguser" {
							return "test1user"
						}
						return "testuser"
					}(),
					func() string {
						if conf == "wrongpass" {
							return "test1pass"
						}
						return "testpass"
					}())
				require.NoError(t, err)

				req := &base.Request{
					Method: base.Announce,
					URL: mustParseURL(func() string {
						if conf == "wrongurl" {
							return "rtsp://myhost/my1path"
						}
						return "rtsp://myhost/mypath"
					}()),
				}
				se.AddAuthorization(req)

				req.URL = mustParseURL("rtsp://myhost/mypath")

				err = va.ValidateRequest(req)

				if conf != "nofail" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	}
}

func TestAuthVLC(t *testing.T) {
	for _, ca := range []struct {
		clientURL string
		serverURL string
	}{
		{
			"rtsp://myhost/mypath/",
			"rtsp://myhost/mypath/trackID=0",
		},
		{
			"rtsp://myhost/mypath/test?testing/",
			"rtsp://myhost/mypath/test?testing/trackID=0",
		},
	} {
		va := NewValidator("testuser", "testpass",
			[]headers.AuthMethod{headers.AuthBasic, headers.AuthDigest})

		se, err := NewSender(va.Header(), "testuser", "testpass")
		require.NoError(t, err)

		req := &base.Request{
			Method: base.Setup,
			URL:    mustParseURL(ca.clientURL),
		}
		se.AddAuthorization(req)

		req.URL = mustParseURL(ca.serverURL)

		err = va.ValidateRequest(req)
		require.NoError(t, err)
	}
}

func TestAuthHashed(t *testing.T) {
	for _, conf := range []string{
		"nofail",
		"wronguser",
		"wrongpass",
	} {
		t.Run(conf, func(t *testing.T) {
			se := NewValidator("sha256:rl3rgi4NcZkpAEcacZnQ2VuOfJ0FxAqCRaKB/SwdZoQ=",
				"sha256:E9JJ8stBJ7QM+nV4ZoUCeHk/gU3tPFh/5YieiJp6n2w=",
				[]headers.AuthMethod{headers.AuthBasic, headers.AuthDigest})

			va, err := NewSender(se.Header(),
				func() string {
					if conf == "wronguser" {
						return "test1user"
					}
					return "testuser"
				}(),
				func() string {
					if conf == "wrongpass" {
						return "test1pass"
					}
					return "testpass"
				}())
			require.NoError(t, err)

			req := &base.Request{
				Method: base.Announce,
				URL:    mustParseURL("rtsp://myhost/mypath"),
			}
			va.AddAuthorization(req)

			err = se.ValidateRequest(req)

			if conf != "nofail" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
