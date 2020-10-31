package auth

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/base"
	"github.com/aler9/gortsplib/headers"
)

var casesAuth = []struct {
	name    string
	methods []headers.AuthMethod
}{
	{
		"basic",
		[]headers.AuthMethod{headers.AuthBasic},
	},
	{
		"digest",
		[]headers.AuthMethod{headers.AuthDigest},
	},
	{
		"both",
		[]headers.AuthMethod{headers.AuthBasic, headers.AuthDigest},
	},
}

func TestAuthMethods(t *testing.T) {
	for _, c := range casesAuth {
		t.Run(c.name, func(t *testing.T) {
			authServer := NewServer("testuser", "testpass", c.methods)
			wwwAuthenticate := authServer.GenerateHeader()

			ac, err := NewClient(wwwAuthenticate, "testuser", "testpass")
			require.NoError(t, err)
			authorization := ac.GenerateHeader(base.ANNOUNCE,
				&url.URL{Scheme: "rtsp", Host: "myhost", Path: "mypath"})

			err = authServer.ValidateHeader(authorization, base.ANNOUNCE,
				&url.URL{Scheme: "rtsp", Host: "myhost", Path: "mypath"})
			require.NoError(t, err)
		})
	}
}

func TestAuthVLC(t *testing.T) {
	authServer := NewServer("testuser", "testpass",
		[]headers.AuthMethod{headers.AuthBasic, headers.AuthDigest})
	wwwAuthenticate := authServer.GenerateHeader()

	ac, err := NewClient(wwwAuthenticate, "testuser", "testpass")
	require.NoError(t, err)
	authorization := ac.GenerateHeader(base.ANNOUNCE,
		&url.URL{Scheme: "rtsp", Host: "myhost", Path: "/mypath/"})

	err = authServer.ValidateHeader(authorization, base.ANNOUNCE,
		&url.URL{Scheme: "rtsp", Host: "myhost", Path: "/mypath/trackId=0"})
	require.NoError(t, err)
}
