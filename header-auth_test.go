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
		`Digest nonce="8b84a3b789283a8bea8da7fa7d41f08b", realm="4419b63f5e51", stale="FALSE"`,
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
		`Digest nonce="8b84a3b789283a8bea8da7fa7d41f08b", realm="4419b63f5e51", stale="FALSE"`,
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
		"digest response",
		`Digest username="aa", realm="bb", nonce="cc", uri="dd", response="ee"`,
		`Digest nonce="cc", realm="bb", response="ee", uri="dd", username="aa"`,
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
