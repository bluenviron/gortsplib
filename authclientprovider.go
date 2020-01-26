package gortsplib

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

func md5Hex(in string) string {
	h := md5.New()
	h.Write([]byte(in))
	return hex.EncodeToString(h.Sum(nil))
}

type authClientProvider struct {
	user  string
	pass  string
	realm string
	nonce string
}

func newAuthClientProvider(user string, pass string, realm string, nonce string) *authClientProvider {
	return &authClientProvider{
		user:  user,
		pass:  pass,
		realm: realm,
		nonce: nonce,
	}
}

func (ap *authClientProvider) generateHeader(method string, path string) string {
	ha1 := md5Hex(ap.user + ":" + ap.realm + ":" + ap.pass)
	ha2 := md5Hex(method + ":" + path)
	response := md5Hex(ha1 + ":" + ap.nonce + ":" + ha2)

	return fmt.Sprintf("Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"%s\", response=\"%s\"",
		ap.user, ap.realm, ap.nonce, path, response)
}
