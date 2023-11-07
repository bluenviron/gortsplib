package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
)

func mustParseURL(s string) *base.URL {
	u, err := base.ParseURL(s)
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
				nonce, err := GenerateNonce()
				require.NoError(t, err)

				se, err := NewSender(
					GenerateWWWAuthenticate(c1.methods, "IPCAM", nonce),
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

				err = Validate(req, "testuser", "testpass", nil, c1.methods, "IPCAM", nonce)

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
		mediaURL  string
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
		nonce, err := GenerateNonce()
		require.NoError(t, err)

		se, err := NewSender(
			GenerateWWWAuthenticate(nil, "IPCAM", nonce),
			"testuser",
			"testpass")
		require.NoError(t, err)

		req := &base.Request{
			Method: base.Setup,
			URL:    mustParseURL(ca.clientURL),
		}
		se.AddAuthorization(req)
		req.URL = mustParseURL(ca.mediaURL)

		err = Validate(req, "testuser", "testpass", mustParseURL(ca.clientURL), nil, "IPCAM", nonce)
		require.NoError(t, err)

		err = Validate(req, "testuser", "testpass", mustParseURL("rtsp://invalid"), nil, "IPCAM", nonce)
		require.Error(t, err)
	}
}
