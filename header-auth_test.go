package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesHeaderAuth = []struct {
	name string
	dec  string
	enc  string
	ha   *HeaderAuth
}{
	{
		"basic",
		`Basic realm="4419b63f5e51"`,
		`Basic realm="4419b63f5e51"`,
		&HeaderAuth{
			Prefix: "Basic",
			Values: map[string]string{
				"realm": "4419b63f5e51",
			},
		},
	},
	{
		"digest request 1",
		`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`,
		`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`,
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
		`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale=FALSE`,
		`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`,
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
		`Digest realm="4419b63f5e51",nonce="133767111917411116111311118211673010032",  stale="FALSE"`,
		`Digest realm="4419b63f5e51", nonce="133767111917411116111311118211673010032", stale="FALSE"`,
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
		"digest response 1",
		`Digest username="aa", realm="bb", nonce="cc", uri="dd", response="ee"`,
		`Digest realm="bb", nonce="cc", response="ee", uri="dd", username="aa"`,
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
		`Digest username="", realm="IPCAM", nonce="5d17cd12b9fa8a85ac5ceef0926ea5a6", uri="rtsp://localhost:8554/mystream", response="c072ae90eb4a27f4cdcb90d62266b2a1"`,
		`Digest realm="IPCAM", nonce="5d17cd12b9fa8a85ac5ceef0926ea5a6", response="c072ae90eb4a27f4cdcb90d62266b2a1", uri="rtsp://localhost:8554/mystream", username=""`,
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
