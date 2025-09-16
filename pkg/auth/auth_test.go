package auth

import (
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

func TestCombined(t *testing.T) {
	for _, c1 := range []struct {
		name    string
		methods []VerifyMethod
	}{
		{
			"basic",
			[]VerifyMethod{VerifyMethodBasic},
		},
		{
			"digest md5",
			[]VerifyMethod{VerifyMethodDigestMD5},
		},
		{
			"digest sha256",
			[]VerifyMethod{VerifyMethodDigestSHA256},
		},
		{
			"all",
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

				se := &Sender{
					WWWAuth: GenerateWWWAuthenticate(c1.methods, "IPCAM", nonce),
					User: func() string {
						if conf == "wronguser" {
							return "test1user"
						}
						return "testuser"
					}(),
					Pass: func() string {
						if conf == "wrongpass" {
							return "test1pass"
						}
						return "testpass"
					}(),
				}
				err = se.Initialize()
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

				err = Verify(req, "testuser", "testpass", c1.methods, "IPCAM", nonce)

				if conf != "nofail" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			})
		}
	}
}
