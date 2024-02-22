package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
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
		base.HeaderValue{`Digest username="Mufasa", realm="testrealm@host.com", ` +
			`nonce="dcd98b7102dd2f0e8b11d0f600bfb0c093", ` +
			`uri="/dir/index.html", response="e966c932a9242554e42c8ee200cec7f6", opaque="5ccc069c403ebaf9f0171e9517f40e41"`},
		base.HeaderValue{`Digest username="Mufasa", realm="testrealm@host.com", ` +
			`nonce="dcd98b7102dd2f0e8b11d0f600bfb0c093", ` +
			`uri="/dir/index.html", response="e966c932a9242554e42c8ee200cec7f6", opaque="5ccc069c403ebaf9f0171e9517f40e41"`},
		Authorization{
			Method:   AuthDigest,
			Username: "Mufasa",
			Realm:    "testrealm@host.com",
			Nonce:    "dcd98b7102dd2f0e8b11d0f600bfb0c093",
			URI:      "/dir/index.html",
			Response: "e966c932a9242554e42c8ee200cec7f6",
			Opaque:   stringPtr("5ccc069c403ebaf9f0171e9517f40e41"),
		},
	},
	{
		"digest with empty field",
		base.HeaderValue{`Digest username="", realm="IPCAM", ` +
			`nonce="5d17cd12b9fa8a85ac5ceef0926ea5a6", uri="rtsp://localhost:8554/mystream", ` +
			`response="c072ae90eb4a27f4cdcb90d62266b2a1"`},
		base.HeaderValue{`Digest username="", realm="IPCAM", ` +
			`nonce="5d17cd12b9fa8a85ac5ceef0926ea5a6", uri="rtsp://localhost:8554/mystream", ` +
			`response="c072ae90eb4a27f4cdcb90d62266b2a1"`},
		Authorization{
			Method:   AuthDigest,
			Username: "",
			Realm:    "IPCAM",
			Nonce:    "5d17cd12b9fa8a85ac5ceef0926ea5a6",
			URI:      "rtsp://localhost:8554/mystream",
			Response: "c072ae90eb4a27f4cdcb90d62266b2a1",
		},
	},
}

func TestAuthorizationUnmarshal(t *testing.T) {
	for _, ca := range casesAuthorization {
		t.Run(ca.name, func(t *testing.T) {
			var h Authorization
			err := h.Unmarshal(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestAuthorizationMarshal(t *testing.T) {
	for _, ca := range casesAuthorization {
		t.Run(ca.name, func(t *testing.T) {
			vout := ca.h.Marshal()
			require.Equal(t, ca.vout, vout)
		})
	}
}

func FuzzAuthorizationUnmarshal(f *testing.F) {
	for _, ca := range casesAuthorization {
		f.Add(ca.vin[0])
	}

	f.Fuzz(func(t *testing.T, b string) {
		var h Authorization
		h.Unmarshal(base.HeaderValue{b}) //nolint:errcheck
	})
}

func TestAuthorizationAdditionalErrors(t *testing.T) {
	func() {
		var h Authorization
		err := h.Unmarshal(base.HeaderValue{})
		require.Error(t, err)
	}()

	func() {
		var h Authorization
		err := h.Unmarshal(base.HeaderValue{"a", "b"})
		require.Error(t, err)
	}()
}
