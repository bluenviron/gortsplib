package auth

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/v2/pkg/base"
)

func TestSenderErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		hv   base.HeaderValue
		err  string
	}{
		{
			"invalid method",
			base.HeaderValue{`Invalid`},
			"no authentication methods available",
		},
		{
			"digest invalid",
			base.HeaderValue{`Digest`},
			"unable to split between method and keys (Digest)",
		},
		{
			"digest, missing realm",
			base.HeaderValue{`Digest nonce=123`},
			"realm is missing",
		},
		{
			"digest, missing nonce",
			base.HeaderValue{`Digest realm=123`},
			"nonce is missing",
		},
		{
			"basic invalid",
			base.HeaderValue{`Basic`},
			"unable to split between method and keys (Basic)",
		},
		{
			"basic, missing realm",
			base.HeaderValue{`Basic nonce=123`},
			"realm is missing",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := NewSender(ca.hv, "myuser", "mypass")
			require.EqualError(t, err, ca.err)
		})
	}
}
