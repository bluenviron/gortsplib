package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesHeaderAuth = []struct {
	name string
	dec  HeaderValue
	enc  HeaderValue
	ha   *HeaderAuth
}{
	{
		"basic",
		HeaderValue{`Basic realm="4419b63f5e51"`},
		HeaderValue{`Basic realm="4419b63f5e51"`},
		&HeaderAuth{
			Prefix: "Basic",
			Values: map[string]string{
				"realm": "4419b63f5e51",
			},
		},
	},
	{
		"digest request 1",
		HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`},
		HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`},
		&HeaderAuth{
			Prefix: "Digest",
			Values: map[string]string{
				"realm": "4419b63f5e51",
				"nonce": "8b84a3b789283a8bea8da7fa7d41f08b",
				"stale": "FALSE",
			},
		},
	},
	{
		"digest request 2",
		HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale=FALSE`},
		HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`},
		&HeaderAuth{
			Prefix: "Digest",
			Values: map[string]string{
				"realm": "4419b63f5e51",
				"nonce": "8b84a3b789283a8bea8da7fa7d41f08b",
				"stale": "FALSE",
			},
		},
	},
	{
		"digest request 3",
		HeaderValue{`Digest realm="4419b63f5e51",nonce="133767111917411116111311118211673010032",  stale="FALSE"`},
		HeaderValue{`Digest realm="4419b63f5e51", nonce="133767111917411116111311118211673010032", stale="FALSE"`},
		&HeaderAuth{
			Prefix: "Digest",
			Values: map[string]string{
				"realm": "4419b63f5e51",
				"nonce": "133767111917411116111311118211673010032",
				"stale": "FALSE",
			},
		},
	},
	{
		"digest response generic",
		HeaderValue{`Digest username="aa", realm="bb", nonce="cc", uri="dd", response="ee"`},
		HeaderValue{`Digest username="aa", realm="bb", nonce="cc", uri="dd", response="ee"`},
		&HeaderAuth{
			Prefix: "Digest",
			Values: map[string]string{
				"username": "aa",
				"realm":    "bb",
				"nonce":    "cc",
				"uri":      "dd",
				"response": "ee",
			},
		},
	},
	{
		"digest response with empty field",
		HeaderValue{`Digest username="", realm="IPCAM", nonce="5d17cd12b9fa8a85ac5ceef0926ea5a6", uri="rtsp://localhost:8554/mystream", response="c072ae90eb4a27f4cdcb90d62266b2a1"`},
		HeaderValue{`Digest username="", realm="IPCAM", nonce="5d17cd12b9fa8a85ac5ceef0926ea5a6", uri="rtsp://localhost:8554/mystream", response="c072ae90eb4a27f4cdcb90d62266b2a1"`},
		&HeaderAuth{
			Prefix: "Digest",
			Values: map[string]string{
				"username": "",
				"realm":    "IPCAM",
				"nonce":    "5d17cd12b9fa8a85ac5ceef0926ea5a6",
				"uri":      "rtsp://localhost:8554/mystream",
				"response": "c072ae90eb4a27f4cdcb90d62266b2a1",
			},
		},
	},
	{
		"digest response with no spaces and additional fields",
		HeaderValue{`Digest realm="Please log in with a valid username",nonce="752a62306daf32b401a41004555c7663",opaque="",stale=FALSE,algorithm=MD5`},
		HeaderValue{`Digest realm="Please log in with a valid username", nonce="752a62306daf32b401a41004555c7663", opaque="", stale="FALSE", algorithm="MD5"`},
		&HeaderAuth{
			Prefix: "Digest",
			Values: map[string]string{
				"realm":     "Please log in with a valid username",
				"nonce":     "752a62306daf32b401a41004555c7663",
				"opaque":    "",
				"stale":     "FALSE",
				"algorithm": "MD5",
			},
		},
	},
}

func TestHeaderAuthRead(t *testing.T) {
	for _, c := range casesHeaderAuth {
		t.Run(c.name, func(t *testing.T) {
			req, err := ReadHeaderAuth(c.dec)
			require.NoError(t, err)
			require.Equal(t, c.ha, req)
		})
	}
}

func TestHeaderAuthWrite(t *testing.T) {
	for _, c := range casesHeaderAuth {
		t.Run(c.name, func(t *testing.T) {
			req := c.ha.Write()
			require.Equal(t, c.enc, req)
		})
	}
}
