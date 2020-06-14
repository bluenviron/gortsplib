package gortsplib

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

var casesAuth = []struct {
	name    string
	methods []AuthMethod
}{
	{
		"basic",
		[]AuthMethod{Basic},
	},
	{
		"digest",
		[]AuthMethod{Digest},
	},
	{
		"both",
		[]AuthMethod{Basic, Digest},
	},
}

func TestAuth(t *testing.T) {
	for _, c := range casesAuth {
		t.Run(c.name, func(t *testing.T) {
			authServer := NewAuthServer("testuser", "testpass", c.methods)
			wwwAuthenticate := authServer.GenerateHeader()

			ac, err := NewAuthClient(wwwAuthenticate, "testuser", "testpass")
			require.NoError(t, err)
			authorization := ac.GenerateHeader(ANNOUNCE,
				&url.URL{Scheme: "rtsp", Host: "myhost", Path: "mypath"})

			err = authServer.ValidateHeader(authorization, ANNOUNCE,
				&url.URL{Scheme: "rtsp", Host: "myhost", Path: "mypath"})
			require.NoError(t, err)
		})
	}
}
