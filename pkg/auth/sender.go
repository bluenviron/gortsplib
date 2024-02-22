package auth

import (
	"fmt"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
)

func findAuthenticateHeader(auths []headers.Authenticate, method headers.AuthMethod) *headers.Authenticate {
	for _, auth := range auths {
		if auth.Method == method {
			return &auth
		}
	}
	return nil
}

func pickAuthenticateHeader(auths []headers.Authenticate) (*headers.Authenticate, error) {
	if auth := findAuthenticateHeader(auths, headers.AuthDigestSHA256); auth != nil {
		return auth, nil
	}

	if auth := findAuthenticateHeader(auths, headers.AuthDigestMD5); auth != nil {
		return auth, nil
	}

	if auth := findAuthenticateHeader(auths, headers.AuthBasic); auth != nil {
		return auth, nil
	}

	return nil, fmt.Errorf("no authentication methods available")
}

// Sender allows to send credentials.
type Sender struct {
	user               string
	pass               string
	authenticateHeader *headers.Authenticate
}

// NewSender allocates a Sender.
// It requires a WWW-Authenticate header (provided by the server)
// and a set of credentials.
func NewSender(vals base.HeaderValue, user string, pass string) (*Sender, error) {
	var auths []headers.Authenticate //nolint:prealloc

	for _, v := range vals {
		var auth headers.Authenticate
		err := auth.Unmarshal(base.HeaderValue{v})
		if err != nil {
			continue // ignore unrecognized headers
		}

		auths = append(auths, auth)
	}

	auth, err := pickAuthenticateHeader(auths)
	if err != nil {
		return nil, err
	}

	return &Sender{
		user:               user,
		pass:               pass,
		authenticateHeader: auth,
	}, nil
}

// AddAuthorization adds the Authorization header to a Request.
func (se *Sender) AddAuthorization(req *base.Request) {
	urStr := req.URL.CloneWithoutCredentials().String()

	h := headers.Authorization{
		Method: se.authenticateHeader.Method,
	}

	switch se.authenticateHeader.Method {
	case headers.AuthBasic:
		h.BasicUser = se.user
		h.BasicPass = se.pass

	case headers.AuthDigestMD5:
		h.Username = se.user
		h.Realm = se.authenticateHeader.Realm
		h.Nonce = se.authenticateHeader.Nonce
		h.URI = urStr
		h.Response = md5Hex(md5Hex(se.user+":"+se.authenticateHeader.Realm+":"+se.pass) + ":" +
			se.authenticateHeader.Nonce + ":" + md5Hex(string(req.Method)+":"+urStr))

	default: // digest SHA-256
		h.Username = se.user
		h.Realm = se.authenticateHeader.Realm
		h.Nonce = se.authenticateHeader.Nonce
		h.URI = urStr
		h.Response = sha256Hex(sha256Hex(se.user+":"+se.authenticateHeader.Realm+":"+se.pass) + ":" +
			se.authenticateHeader.Nonce + ":" + sha256Hex(string(req.Method)+":"+urStr))
	}

	if req.Header == nil {
		req.Header = make(base.Header)
	}

	req.Header["Authorization"] = h.Marshal()
}
