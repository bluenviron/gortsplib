package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesHeaderAuthRequest = []struct {
	name string
	byts string
	har  *HeaderAuthRequest
}{
	{
		"basic",
		`Basic realm="4419b63f5e51"`,
		&HeaderAuthRequest{
			Prefix: "Basic",
			Values: map[string]string{
				"realm": "4419b63f5e51",
			},
		},
	},
	{
		"digest",
		`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`,
		&HeaderAuthRequest{
			Prefix: "Digest",
			Values: map[string]string{
				"realm": "4419b63f5e51",
				"nonce": "8b84a3b789283a8bea8da7fa7d41f08b",
				"stale": "FALSE",
			},
		},
	},
}

func TestHeaderAuthRequest(t *testing.T) {
	for _, c := range casesHeaderAuthRequest {
		t.Run(c.name, func(t *testing.T) {
			req, err := ReadHeaderAuthRequest(c.byts)
			require.NoError(t, err)
			require.Equal(t, c.har, req)
		})
	}
}
