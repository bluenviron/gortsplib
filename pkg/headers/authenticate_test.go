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
			Realm:  stringPtr("4419b63f5e51"),
		},
	},
	{
		"digest request 1",
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`},
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`},
		Authenticate{
			Method: AuthDigest,
			Realm:  stringPtr("4419b63f5e51"),
			Nonce:  stringPtr("8b84a3b789283a8bea8da7fa7d41f08b"),
			Stale:  stringPtr("FALSE"),
		},
	},
	{
		"digest request 2",
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale=FALSE`},
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`},
		Authenticate{
			Method: AuthDigest,
			Realm:  stringPtr("4419b63f5e51"),
			Nonce:  stringPtr("8b84a3b789283a8bea8da7fa7d41f08b"),
			Stale:  stringPtr("FALSE"),
		},
	},
	{
		"digest request 3",
		base.HeaderValue{`Digest realm="4419b63f5e51",nonce="133767111917411116111311118211673010032",  stale="FALSE"`},
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="133767111917411116111311118211673010032", stale="FALSE"`},
		Authenticate{
			Method: AuthDigest,
			Realm:  stringPtr("4419b63f5e51"),
			Nonce:  stringPtr("133767111917411116111311118211673010032"),
			Stale:  stringPtr("FALSE"),
		},
	},
	{
		"digest response generic",
		base.HeaderValue{`Digest username="aa", realm="bb", nonce="cc", uri="dd", response="ee"`},
		base.HeaderValue{`Digest username="aa", realm="bb", nonce="cc", uri="dd", response="ee"`},
		Authenticate{
			Method:   AuthDigest,
			Username: stringPtr("aa"),
			Realm:    stringPtr("bb"),
			Nonce:    stringPtr("cc"),
			URI:      stringPtr("dd"),
			Response: stringPtr("ee"),
		},
	},
	{
		"digest response with empty field",
		base.HeaderValue{`Digest username="", realm="IPCAM", ` +
			`nonce="5d17cd12b9fa8a85ac5ceef0926ea5a6", uri="rtsp://localhost:8554/mystream", ` +
			`response="c072ae90eb4a27f4cdcb90d62266b2a1"`},
		base.HeaderValue{`Digest username="", realm="IPCAM", ` +
			`nonce="5d17cd12b9fa8a85ac5ceef0926ea5a6", uri="rtsp://localhost:8554/mystream", ` +
			`response="c072ae90eb4a27f4cdcb90d62266b2a1"`},
		Authenticate{
			Method:   AuthDigest,
			Username: stringPtr(""),
			Realm:    stringPtr("IPCAM"),
			Nonce:    stringPtr("5d17cd12b9fa8a85ac5ceef0926ea5a6"),
			URI:      stringPtr("rtsp://localhost:8554/mystream"),
			Response: stringPtr("c072ae90eb4a27f4cdcb90d62266b2a1"),
		},
	},
	{
		"digest response with no spaces and additional fields",
		base.HeaderValue{`Digest realm="Please log in with a valid username",` +
			`nonce="752a62306daf32b401a41004555c7663",opaque="",stale=FALSE,algorithm=MD5`},
		base.HeaderValue{`Digest realm="Please log in with a valid username", ` +
			`nonce="752a62306daf32b401a41004555c7663", opaque="", stale="FALSE", algorithm="MD5"`},
		Authenticate{
			Method:    AuthDigest,
			Realm:     stringPtr("Please log in with a valid username"),
			Nonce:     stringPtr("752a62306daf32b401a41004555c7663"),
			Opaque:    stringPtr(""),
			Stale:     stringPtr("FALSE"),
			Algorithm: stringPtr("MD5"),
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

func TestAutenticatehUnmarshalErrors(t *testing.T) {
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
			"no keys",
			base.HeaderValue{"Basic"},
			"unable to split between method and keys (Basic)",
		},
		{
			"invalid keys",
			base.HeaderValue{`Basic key1="k`},
			"apexes not closed (key1=\"k)",
		},
		{
			"invalid method",
			base.HeaderValue{"Testing key1=val1"},
			"invalid method (Testing)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var h Authenticate
			err := h.Unmarshal(ca.hv)
			require.EqualError(t, err, ca.err)
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
