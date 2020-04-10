package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAuthClientServer(t *testing.T) {
	as := NewAuthServer("testuser", "testpass")
	wwwAuthenticate := as.GenerateHeader()

	ac, err := NewAuthClient(wwwAuthenticate, "testuser", "testpass")
	require.NoError(t, err)
	authorization := ac.GenerateHeader("ANNOUNCE", "rtsp://myhost/mypath")

	err = as.ValidateHeader(authorization, "ANNOUNCE", "rtsp://myhost/mypath")
	require.NoError(t, err)
}
