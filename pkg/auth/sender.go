package auth

import (
	"fmt"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
)

// Sender allows to send credentials.
type Sender struct {
	user       string
	pass       string
	authHeader *headers.Authenticate
}

// NewSender allocates a Sender.
// It requires a WWW-Authenticate header (provided by the server)
// and a set of credentials.
func NewSender(wwwAuth base.HeaderValue, user string, pass string) (*Sender, error) {
	var bestAuthHeader *headers.Authenticate

	for _, v := range wwwAuth {
		var auth headers.Authenticate
		err := auth.Unmarshal(base.HeaderValue{v})
		if err != nil {
			continue // ignore unrecognized headers
		}

		if bestAuthHeader == nil ||
			(auth.Algorithm != nil && *auth.Algorithm == headers.AuthAlgorithmSHA256) ||
			(bestAuthHeader.Method == headers.AuthMethodBasic) {
			bestAuthHeader = &auth
		}
	}

	if bestAuthHeader == nil {
		return nil, fmt.Errorf("no authentication methods available")
	}

	return &Sender{
		user:       user,
		pass:       pass,
		authHeader: bestAuthHeader,
	}, nil
}

// AddAuthorization adds the Authorization header to a Request.
func (se *Sender) AddAuthorization(req *base.Request) {
	urStr := req.URL.CloneWithoutCredentials().String()

	h := headers.Authorization{
		Method: se.authHeader.Method,
	}

	h.Username = se.user

	if se.authHeader.Method == headers.AuthMethodBasic {
		h.BasicPass = se.pass
	} else { // digest
		h.Realm = se.authHeader.Realm
		h.Nonce = se.authHeader.Nonce
		h.URI = urStr
		h.Algorithm = se.authHeader.Algorithm

		if se.authHeader.Algorithm == nil || *se.authHeader.Algorithm == headers.AuthAlgorithmMD5 {
			h.Response = md5Hex(md5Hex(se.user+":"+se.authHeader.Realm+":"+se.pass) + ":" +
				se.authHeader.Nonce + ":" + md5Hex(string(req.Method)+":"+urStr))
		} else { // sha256
			h.Response = sha256Hex(sha256Hex(se.user+":"+se.authHeader.Realm+":"+se.pass) + ":" +
				se.authHeader.Nonce + ":" + sha256Hex(string(req.Method)+":"+urStr))
		}
	}

	if req.Header == nil {
		req.Header = make(base.Header)
	}

	req.Header["Authorization"] = h.Marshal()
}
