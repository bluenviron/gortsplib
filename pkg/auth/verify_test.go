package auth

import (
	"testing"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/stretchr/testify/require"
)

var casesVerify = []struct {
	name          string
	authorization base.HeaderValue
}{
	{
		"basic",
		base.HeaderValue{
			"Basic bXl1c2VyOm15cGFzcw==",
		},
	},
	{
		"digest md5 implicit",
		base.HeaderValue{
			"Digest username=\"myuser\", realm=\"myrealm\", nonce=\"f49ac6dd0ba708d4becddc9692d1f2ce\", " +
				"uri=\"rtsp://myhost/mypath?key=val/trackID=3\", response=\"ba6e9cccbfeb38db775378a0a9067ba5\"",
		},
	},
	{
		"digest md5 explicit",
		base.HeaderValue{
			"Digest username=\"myuser\", realm=\"myrealm\", nonce=\"f49ac6dd0ba708d4becddc9692d1f2ce\", " +
				"uri=\"rtsp://myhost/mypath?key=val/trackID=3\", response=\"ba6e9cccbfeb38db775378a0a9067ba5\", " +
				"algorithm=\"MD5\"",
		},
	},
	{
		"digest sha256",
		base.HeaderValue{
			"Digest username=\"myuser\", realm=\"myrealm\", nonce=\"f49ac6dd0ba708d4becddc9692d1f2ce\", " +
				"uri=\"rtsp://myhost/mypath?key=val/trackID=3\", " +
				"response=\"e298296ce35c9ab79699c8f3f9508944c1be9395e892f8205b6d66f1b8e663ee\", " +
				"algorithm=\"SHA-256\"",
		},
	},
	{
		"digest vlc",
		base.HeaderValue{
			"Digest username=\"myuser\", realm=\"myrealm\", nonce=\"f49ac6dd0ba708d4becddc9692d1f2ce\", " +
				"uri=\"rtsp://myhost/mypath?key=val/\", response=\"5ca5ceeca20a05e9a3f49ecde4b42655\"",
		},
	},
}

func TestVerify(t *testing.T) {
	for _, ca := range casesVerify {
		t.Run(ca.name, func(t *testing.T) {
			req := &base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://myhost/mypath?key=val/trackID=3"),
				Header: base.Header{
					"Authorization": ca.authorization,
				},
			}

			err := Verify(
				req,
				"myuser",
				"mypass",
				[]VerifyMethod{VerifyMethodBasic, VerifyMethodDigestMD5, VerifyMethodDigestSHA256},
				"myrealm",
				"f49ac6dd0ba708d4becddc9692d1f2ce")
			require.NoError(t, err)
		})
	}
}

func FuzzVerify(f *testing.F) {
	for _, ca := range casesVerify {
		f.Add(ca.authorization[0])
	}

	f.Fuzz(func(_ *testing.T, a string) {
		Verify( //nolint:errcheck
			&base.Request{
				Method: base.Describe,
				URL:    nil,
				Header: base.Header{
					"Authorization": base.HeaderValue{a},
				},
			},
			"myuser",
			"mypass",
			nil,
			"IPCAM",
			"f49ac6dd0ba708d4becddc9692d1f2ce",
		)
	})
}
