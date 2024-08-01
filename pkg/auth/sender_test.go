package auth

import (
	"testing"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/stretchr/testify/require"
)

var casesSender = []struct {
	name            string
	wwwAuthenticate base.HeaderValue
	authorization   base.HeaderValue
}{
	{
		"basic",
		base.HeaderValue{
			"Basic realm=testrealm",
		},
		base.HeaderValue{
			"Basic bXl1c2VyOm15cGFzcw==",
		},
	},
	{
		"digest md5 implicit",
		base.HeaderValue{
			`Digest realm="myrealm", nonce="f49ac6dd0ba708d4becddc9692d1f2ce"`,
		},
		base.HeaderValue{
			"Digest username=\"myuser\", realm=\"myrealm\", nonce=\"f49ac6dd0ba708d4becddc9692d1f2ce\", " +
				"uri=\"rtsp://myhost/mypath?key=val/trackID=3\", response=\"ba6e9cccbfeb38db775378a0a9067ba5\"",
		},
	},
	{
		"digest md5 explicit",
		base.HeaderValue{
			`Digest realm="myrealm", nonce="f49ac6dd0ba708d4becddc9692d1f2ce", algorithm="MD5"`,
		},
		base.HeaderValue{
			"Digest username=\"myuser\", realm=\"myrealm\", nonce=\"f49ac6dd0ba708d4becddc9692d1f2ce\", " +
				"uri=\"rtsp://myhost/mypath?key=val/trackID=3\", response=\"ba6e9cccbfeb38db775378a0a9067ba5\", " +
				"algorithm=\"MD5\"",
		},
	},
	{
		"digest sha256",
		base.HeaderValue{
			`Digest realm="myrealm", nonce="f49ac6dd0ba708d4becddc9692d1f2ce", algorithm="SHA-256"`,
		},
		base.HeaderValue{
			"Digest username=\"myuser\", realm=\"myrealm\", nonce=\"f49ac6dd0ba708d4becddc9692d1f2ce\", " +
				"uri=\"rtsp://myhost/mypath?key=val/trackID=3\", " +
				"response=\"e298296ce35c9ab79699c8f3f9508944c1be9395e892f8205b6d66f1b8e663ee\", " +
				"algorithm=\"SHA-256\"",
		},
	},
	{
		"multiple 1",
		base.HeaderValue{
			"Basic realm=testrealm",
			`Digest realm="myrealm", nonce="f49ac6dd0ba708d4becddc9692d1f2ce"`,
		},
		base.HeaderValue{
			"Digest username=\"myuser\", realm=\"myrealm\", nonce=\"f49ac6dd0ba708d4becddc9692d1f2ce\", " +
				"uri=\"rtsp://myhost/mypath?key=val/trackID=3\", response=\"ba6e9cccbfeb38db775378a0a9067ba5\"",
		},
	},
	{
		"multiple 2",
		base.HeaderValue{
			"Basic realm=testrealm",
			`Digest realm="myrealm", nonce="f49ac6dd0ba708d4becddc9692d1f2ce", algorithm="MD5"`,
			`Digest realm="myrealm", nonce="f49ac6dd0ba708d4becddc9692d1f2ce", algorithm="SHA-256"`,
		},
		base.HeaderValue{
			"Digest username=\"myuser\", realm=\"myrealm\", nonce=\"f49ac6dd0ba708d4becddc9692d1f2ce\", " +
				"uri=\"rtsp://myhost/mypath?key=val/trackID=3\", " +
				"response=\"e298296ce35c9ab79699c8f3f9508944c1be9395e892f8205b6d66f1b8e663ee\", " +
				"algorithm=\"SHA-256\"",
		},
	},
}

func TestSender(t *testing.T) {
	for _, ca := range casesSender {
		t.Run(ca.name, func(t *testing.T) {
			se, err := NewSender(ca.wwwAuthenticate, "myuser", "mypass")
			require.NoError(t, err)

			req := &base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://myhost/mypath?key=val/trackID=3"),
			}
			se.AddAuthorization(req)

			require.Equal(t, ca.authorization, req.Header["Authorization"])
		})
	}
}

func FuzzSender(f *testing.F) {
	for _, ca := range casesSender {
		f.Add(ca.authorization[0])
	}

	f.Fuzz(func(_ *testing.T, a string) {
		se, err := NewSender(base.HeaderValue{a}, "myuser", "mypass")
		if err == nil {
			se.AddAuthorization(&base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://myhost/mypath?key=val/trackID=3"),
			})
		}
	})
}
