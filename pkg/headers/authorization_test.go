package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
)

var casesAuthorization = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    Authorization
}{
	{
		"basic",
		base.HeaderValue{"Basic bXl1c2VyOm15cGFzcw=="},
		base.HeaderValue{"Basic bXl1c2VyOm15cGFzcw=="},
		Authorization{
			Method:    AuthBasic,
			BasicUser: "myuser",
			BasicPass: "mypass",
		},
	},
	{
		"digest",
		base.HeaderValue{"Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\""},
		base.HeaderValue{"Digest realm=\"4419b63f5e51\", nonce=\"8b84a3b789283a8bea8da7fa7d41f08b\", stale=\"FALSE\""},
		Authorization{
			Method: AuthDigest,
			DigestValues: Authenticate{
				Method: AuthDigest,
				Realm: func() *string {
					v := "4419b63f5e51"
					return &v
				}(),
				Nonce: func() *string {
					v := "8b84a3b789283a8bea8da7fa7d41f08b"
					return &v
				}(),
				Stale: func() *string {
					v := "FALSE"
					return &v
				}(),
			},
		},
	},
}

func TestAuthorizationRead(t *testing.T) {
	for _, ca := range casesAuthorization {
		t.Run(ca.name, func(t *testing.T) {
			var h Authorization
			err := h.Read(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestAuthorizationReadErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		hv   base.HeaderValue
		err  string
	}{
		{
			"empty",
			base.HeaderValue{},
			"value not provided",
		},
		{
			"2 values",
			base.HeaderValue{"a", "b"},
			"value provided multiple times ([a b])",
		},
		{
			"invalid",
			base.HeaderValue{`Invalid`},
			"invalid authorization header",
		},
		{
			"basic invalid 1",
			base.HeaderValue{`Basic aaa`},
			"invalid value",
		},
		{
			"basic invalid 2",
			base.HeaderValue{`Basic aW52YWxpZA==`},
			"invalid value",
		},
		{
			"digest invalid",
			base.HeaderValue{`Digest test="v`},
			"apexes not closed (test=\"v)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var h Authorization
			err := h.Read(ca.hv)
			require.EqualError(t, err, ca.err)
		})
	}
}

func TestAuthorizationWrite(t *testing.T) {
	for _, ca := range casesAuthorization {
		t.Run(ca.name, func(t *testing.T) {
			vout := ca.h.Write()
			require.Equal(t, ca.vout, vout)
		})
	}
}
