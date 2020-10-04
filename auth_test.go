package gortsplib

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/base"
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

func TestAuthMethods(t *testing.T) {
	for _, c := range casesAuth {
		t.Run(c.name, func(t *testing.T) {
			authServer := NewAuthServer("testuser", "testpass", c.methods)
			wwwAuthenticate := authServer.GenerateHeader()

			ac, err := newAuthClient(wwwAuthenticate, "testuser", "testpass")
			require.NoError(t, err)
			authorization := ac.GenerateHeader(base.ANNOUNCE,
				&url.URL{Scheme: "rtsp", Host: "myhost", Path: "mypath"})

			err = authServer.ValidateHeader(authorization, base.ANNOUNCE,
				&url.URL{Scheme: "rtsp", Host: "myhost", Path: "mypath"})
			require.NoError(t, err)
		})
	}
}

func TestAuthBasePath(t *testing.T) {
	authServer := NewAuthServer("testuser", "testpass", []AuthMethod{Basic, Digest})
	wwwAuthenticate := authServer.GenerateHeader()

	ac, err := newAuthClient(wwwAuthenticate, "testuser", "testpass")
	require.NoError(t, err)
	authorization := ac.GenerateHeader(base.ANNOUNCE,
		&url.URL{Scheme: "rtsp", Host: "myhost", Path: "mypath/"})

	err = authServer.ValidateHeader(authorization, base.ANNOUNCE,
		&url.URL{Scheme: "rtsp", Host: "myhost", Path: "mypath/trackId=0"})
	require.NoError(t, err)
}
