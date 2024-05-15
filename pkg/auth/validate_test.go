package auth

import (
	"fmt"
	"testing"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	for _, ca := range []struct {
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
	} {
		t.Run(ca.name, func(t *testing.T) {
			se, err := NewSender(
				GenerateWWWAuthenticate([]ValidateMethod{ValidateMethodDigestMD5}, "myrealm", "f49ac6dd0ba708d4becddc9692d1f2ce"),
				"myuser",
				"mypass")
			require.NoError(t, err)
			req1 := &base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://myhost/mypath?key=val/"),
			}
			se.AddAuthorization(req1)
			fmt.Println(req1.Header)

			req := &base.Request{
				Method: base.Setup,
				URL:    mustParseURL("rtsp://myhost/mypath?key=val/trackID=3"),
				Header: base.Header{
					"Authorization": ca.authorization,
				},
			}
			err = Validate(
				req,
				"myuser",
				"mypass",
				nil,
				"myrealm",
				"f49ac6dd0ba708d4becddc9692d1f2ce")
			require.NoError(t, err)
		})
	}
}

func FuzzValidate(f *testing.F) {
	f.Add(`Invalid`)
	f.Add(`Digest `)
	f.Add(`Digest realm=123`)
	f.Add(`Digest realm=123,nonce=123`)
	f.Add(`Digest realm=123,nonce=123,username=123`)
	f.Add(`Digest realm=123,nonce=123,username=123,uri=123`)
	f.Add(`Digest realm=123,nonce=123,username=123,uri=123,response=123`)
	f.Add(`Digest realm=123,nonce=abcde,username=123,uri=123,response=123`)

	f.Fuzz(func(_ *testing.T, a string) {
		Validate( //nolint:errcheck
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
			"abcde",
		)
	})
}
