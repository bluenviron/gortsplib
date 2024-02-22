package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
)

func stringPtr(v string) *string {
	return &v
}

var casesAuthenticate = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    Authenticate
}{
	{
		"basic",
		base.HeaderValue{`Basic realm="4419b63f5e51"`},
		base.HeaderValue{`Basic realm="4419b63f5e51"`},
		Authenticate{
			Method: AuthBasic,
			Realm:  "4419b63f5e51",
		},
	},
	{
		"digest 1",
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`},
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", ` +
			`stale="FALSE", algorithm="MD5"`},
		Authenticate{
			Method: AuthDigestMD5,
			Realm:  "4419b63f5e51",
			Nonce:  "8b84a3b789283a8bea8da7fa7d41f08b",
			Stale:  stringPtr("FALSE"),
		},
	},
	{
		"digest 2",
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale=FALSE`},
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", ` +
			`stale="FALSE", algorithm="MD5"`},
		Authenticate{
			Method: AuthDigestMD5,
			Realm:  "4419b63f5e51",
			Nonce:  "8b84a3b789283a8bea8da7fa7d41f08b",
			Stale:  stringPtr("FALSE"),
		},
	},
	{
		"digest 3",
		base.HeaderValue{`Digest realm="4419b63f5e51",nonce="133767111917411116111311118211673010032",  stale="FALSE"`},
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="133767111917411116111311118211673010032", ` +
			`stale="FALSE", algorithm="MD5"`},
		Authenticate{
			Method: AuthDigestMD5,
			Realm:  "4419b63f5e51",
			Nonce:  "133767111917411116111311118211673010032",
			Stale:  stringPtr("FALSE"),
		},
	},
	{
		"digest after failed auth",
		base.HeaderValue{`Digest realm="Please log in with a valid username",` +
			`nonce="752a62306daf32b401a41004555c7663",opaque="",stale=FALSE,algorithm=MD5`},
		base.HeaderValue{`Digest realm="Please log in with a valid username", ` +
			`nonce="752a62306daf32b401a41004555c7663", opaque="", stale="FALSE", algorithm="MD5"`},
		Authenticate{
			Method: AuthDigestMD5,
			Realm:  "Please log in with a valid username",
			Nonce:  "752a62306daf32b401a41004555c7663",
			Opaque: stringPtr(""),
			Stale:  stringPtr("FALSE"),
		},
	},
	{
		"digest sha256",
		base.HeaderValue{`Digest realm="IP Camera(AB705)", ` +
			`nonce="fcc86deace979a488b2bfb89f4d0812c", algorithm="SHA-256", stale="FALSE"`},
		base.HeaderValue{`Digest realm="IP Camera(AB705)", ` +
			`nonce="fcc86deace979a488b2bfb89f4d0812c", stale="FALSE", algorithm="SHA-256"`},
		Authenticate{
			Method: AuthDigestSHA256,
			Realm:  "IP Camera(AB705)",
			Nonce:  "fcc86deace979a488b2bfb89f4d0812c",
			Stale:  stringPtr("FALSE"),
		},
	},
}

func TestAuthenticateUnmarshal(t *testing.T) {
	for _, ca := range casesAuthenticate {
		t.Run(ca.name, func(t *testing.T) {
			var h Authenticate
			err := h.Unmarshal(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestAuthenticateMarshal(t *testing.T) {
	for _, ca := range casesAuthenticate {
		t.Run(ca.name, func(t *testing.T) {
			vout := ca.h.Marshal()
			require.Equal(t, ca.vout, vout)
		})
	}
}

func FuzzAuthenticateUnmarshal(f *testing.F) {
	for _, ca := range casesAuthenticate {
		f.Add(ca.vin[0])
	}

	f.Fuzz(func(t *testing.T, b string) {
		var h Authenticate
		h.Unmarshal(base.HeaderValue{b}) //nolint:errcheck
	})
}

func TestAuthenticateAdditionalErrors(t *testing.T) {
	func() {
		var h Authenticate
		err := h.Unmarshal(base.HeaderValue{})
		require.Error(t, err)
	}()

	func() {
		var h Authenticate
		err := h.Unmarshal(base.HeaderValue{"a", "b"})
		require.Error(t, err)
	}()
}
