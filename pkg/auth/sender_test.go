package auth_test

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/auth"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
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
			se := &auth.Sender{
				WWWAuth: ca.wwwAuthenticate,
				User:    "myuser",
				Pass:    "mypass",
			}
			err := se.Initialize()
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
		se := &auth.Sender{
			WWWAuth: base.HeaderValue{a},
			User:    "myuser",
			Pass:    "mypass",
		}
		err := se.Initialize()
		if err != nil {
			return
		}

		se.AddAuthorization(&base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://myhost/mypath?key=val/trackID=3"),
		})
	})
}

func testMD5(in string) string {
	h := md5.Sum([]byte(in))
	return hex.EncodeToString(h[:])
}

func testSHA256(in string) string {
	h := sha256.Sum256([]byte(in))
	return hex.EncodeToString(h[:])
}

func TestSenderQop(t *testing.T) {
	const nonce = "f49ac6dd0ba708d4becddc9692d1f2ce"
	const uri = "rtsp://myhost/mypath?key=val/trackID=3"

	for _, ca := range []struct {
		name string
		www  string
		hash func(string) string
	}{
		{
			"md5",
			`Digest realm="myrealm", nonce="` + nonce + `", qop="auth", algorithm="MD5"`,
			testMD5,
		},
		{
			"sha256",
			`Digest realm="myrealm", nonce="` + nonce + `", qop="auth", algorithm="SHA-256"`,
			testSHA256,
		},
		{
			"multiple qop values",
			`Digest realm="myrealm", nonce="` + nonce + `", qop="auth-int,auth"`,
			testMD5,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			se := &auth.Sender{
				WWWAuth: base.HeaderValue{ca.www},
				User:    "myuser",
				Pass:    "mypass",
			}
			err := se.Initialize()
			require.NoError(t, err)

			req := &base.Request{
				Method: base.Setup,
				URL:    mustParseURL(uri),
			}
			se.AddAuthorization(req)

			var h headers.Authorization
			err = h.Unmarshal(req.Header["Authorization"])
			require.NoError(t, err)

			require.Equal(t, "auth", *h.Qop)
			require.Equal(t, "00000001", *h.Nc)
			require.NotEmpty(t, *h.Cnonce)

			// RFC 7616, section 3.4.1
			ha1 := ca.hash("myuser:myrealm:mypass")
			ha2 := ca.hash("SETUP:" + uri)
			require.Equal(t, ca.hash(ha1+":"+nonce+":00000001:"+*h.Cnonce+":auth:"+ha2), h.Response)
		})
	}
}

func TestSenderQopNonceCount(t *testing.T) {
	se := &auth.Sender{
		WWWAuth: base.HeaderValue{
			`Digest realm="myrealm", nonce="f49ac6dd0ba708d4becddc9692d1f2ce", qop="auth"`,
		},
		User: "myuser",
		Pass: "mypass",
	}
	err := se.Initialize()
	require.NoError(t, err)

	var cnonce string

	for i, expected := range []string{"00000001", "00000002", "00000003"} {
		req := &base.Request{
			Method: base.Setup,
			URL:    mustParseURL("rtsp://myhost/mypath"),
		}
		se.AddAuthorization(req)

		var h headers.Authorization
		err = h.Unmarshal(req.Header["Authorization"])
		require.NoError(t, err)
		require.Equal(t, expected, *h.Nc)

		// the client nonce stays the same for the whole session.
		if i == 0 {
			cnonce = *h.Cnonce
		} else {
			require.Equal(t, cnonce, *h.Cnonce)
		}
	}
}

func TestSenderOpaque(t *testing.T) {
	se := &auth.Sender{
		WWWAuth: base.HeaderValue{
			`Digest realm="myrealm", nonce="f49ac6dd0ba708d4becddc9692d1f2ce", ` +
				`opaque="5ccc069c403ebaf9f0171e9517f40e41"`,
		},
		User: "myuser",
		Pass: "mypass",
	}
	err := se.Initialize()
	require.NoError(t, err)

	req := &base.Request{
		Method: base.Setup,
		URL:    mustParseURL("rtsp://myhost/mypath"),
	}
	se.AddAuthorization(req)

	var h headers.Authorization
	err = h.Unmarshal(req.Header["Authorization"])
	require.NoError(t, err)
	require.Equal(t, "5ccc069c403ebaf9f0171e9517f40e41", *h.Opaque)
}
