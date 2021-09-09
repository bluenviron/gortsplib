package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/base"
)

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
			Realm: func() *string {
				v := "4419b63f5e51"
				return &v
			}(),
		},
	},
	{
		"digest request 1",
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`},
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`},
		Authenticate{
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
	{
		"digest request 2",
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale=FALSE`},
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="8b84a3b789283a8bea8da7fa7d41f08b", stale="FALSE"`},
		Authenticate{
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
	{
		"digest request 3",
		base.HeaderValue{`Digest realm="4419b63f5e51",nonce="133767111917411116111311118211673010032",  stale="FALSE"`},
		base.HeaderValue{`Digest realm="4419b63f5e51", nonce="133767111917411116111311118211673010032", stale="FALSE"`},
		Authenticate{
			Method: AuthDigest,
			Realm: func() *string {
				v := "4419b63f5e51"
				return &v
			}(),
			Nonce: func() *string {
				v := "133767111917411116111311118211673010032"
				return &v
			}(),
			Stale: func() *string {
				v := "FALSE"
				return &v
			}(),
		},
	},
	{
		"digest response generic",
		base.HeaderValue{`Digest username="aa", realm="bb", nonce="cc", uri="dd", response="ee"`},
		base.HeaderValue{`Digest username="aa", realm="bb", nonce="cc", uri="dd", response="ee"`},
		Authenticate{
			Method: AuthDigest,
			Username: func() *string {
				v := "aa"
				return &v
			}(),
			Realm: func() *string {
				v := "bb"
				return &v
			}(),
			Nonce: func() *string {
				v := "cc"
				return &v
			}(),
			URI: func() *string {
				v := "dd"
				return &v
			}(),
			Response: func() *string {
				v := "ee"
				return &v
			}(),
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
			Method: AuthDigest,
			Username: func() *string {
				v := ""
				return &v
			}(),
			Realm: func() *string {
				v := "IPCAM"
				return &v
			}(),
			Nonce: func() *string {
				v := "5d17cd12b9fa8a85ac5ceef0926ea5a6"
				return &v
			}(),
			URI: func() *string {
				v := "rtsp://localhost:8554/mystream"
				return &v
			}(),
			Response: func() *string {
				v := "c072ae90eb4a27f4cdcb90d62266b2a1"
				return &v
			}(),
		},
	},
	{
		"digest response with no spaces and additional fields",
		base.HeaderValue{`Digest realm="Please log in with a valid username",` +
			`nonce="752a62306daf32b401a41004555c7663",opaque="",stale=FALSE,algorithm=MD5`},
		base.HeaderValue{`Digest realm="Please log in with a valid username", ` +
			`nonce="752a62306daf32b401a41004555c7663", opaque="", stale="FALSE", algorithm="MD5"`},
		Authenticate{
			Method: AuthDigest,
			Realm: func() *string {
				v := "Please log in with a valid username"
				return &v
			}(),
			Nonce: func() *string {
				v := "752a62306daf32b401a41004555c7663"
				return &v
			}(),
			Opaque: func() *string {
				v := ""
				return &v
			}(),
			Stale: func() *string {
				v := "FALSE"
				return &v
			}(),
			Algorithm: func() *string {
				v := "MD5"
				return &v
			}(),
		},
	},
}

func TestAuthenticateRead(t *testing.T) {
	for _, ca := range casesAuthenticate {
		t.Run(ca.name, func(t *testing.T) {
			var h Authenticate
			err := h.Read(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestAutenticatehReadErrors(t *testing.T) {
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
			err := h.Read(ca.hv)
			require.Equal(t, ca.err, err.Error())
		})
	}
}

func TestAuthenticateWrite(t *testing.T) {
	for _, ca := range casesAuthenticate {
		t.Run(ca.name, func(t *testing.T) {
			vout := ca.h.Write()
			require.Equal(t, ca.vout, vout)
		})
	}
}
