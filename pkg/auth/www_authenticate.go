package auth

import (
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
)

// GenerateWWWAuthenticate generates a WWW-Authenticate header.
func GenerateWWWAuthenticate(methods []headers.AuthMethod, realm string, nonce string) base.HeaderValue {
	if methods == nil {
		methods = []headers.AuthMethod{headers.AuthDigestSHA256, headers.AuthDigestMD5, headers.AuthBasic}
	}

	var ret base.HeaderValue
	for _, m := range methods {
		ret = append(ret, headers.Authenticate{
			Method: m,
			Realm:  realm,
			Nonce:  nonce, // used only by digest
		}.Marshal()...)
	}
	return ret
}
