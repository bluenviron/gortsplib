package gortsplib

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthClientServer(t *testing.T) {
	as := NewAuthServer("testuser", "testpass")
	wwwAuthenticate := as.GenerateHeader()

	ac, err := NewAuthClient(wwwAuthenticate, "testuser", "testpass")
	require.NoError(t, err)
	authorization := ac.GenerateHeader(ANNOUNCE,
		&url.URL{Scheme: "rtsp", Host: "myhost", Path: "mypath"})

	err = as.ValidateHeader(authorization, ANNOUNCE,
		&url.URL{Scheme: "rtsp", Host: "myhost", Path: "mypath"})
	require.NoError(t, err)
}
