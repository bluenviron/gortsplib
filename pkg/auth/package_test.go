package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
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
			va := NewValidator("testuser", "testpass", c.methods)
			wwwAuthenticate := va.GenerateHeader()

			se, err := NewSender(wwwAuthenticate, "testuser", "testpass")
			require.NoError(t, err)
			authorization := se.GenerateHeader(base.Announce,
				base.MustParseURL("rtsp://myhost/mypath"))

			err = va.ValidateHeader(authorization, base.Announce,
				base.MustParseURL("rtsp://myhost/mypath"))
			require.NoError(t, err)
		})
	}
}

func TestAuthVLC(t *testing.T) {
	for _, ca := range []struct {
		clientURL string
		serverURL string
	}{
		{
			"rtsp://myhost/mypath/",
			"rtsp://myhost/mypath/trackID=0",
		},
		{
			"rtsp://myhost/mypath/test?testing/",
			"rtsp://myhost/mypath/test?testing/trackID=0",
		},
	} {
		se := NewValidator("testuser", "testpass",
			[]headers.AuthMethod{headers.AuthBasic, headers.AuthDigest})

		va, err := NewSender(se.GenerateHeader(), "testuser", "testpass")
		require.NoError(t, err)
		authorization := va.GenerateHeader(base.Announce,
			base.MustParseURL(ca.clientURL))

		err = se.ValidateHeader(authorization, base.Announce,
			base.MustParseURL(ca.serverURL))
		require.NoError(t, err)
	}
}
