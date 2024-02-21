package auth

import (
	"fmt"
	"strings"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
)

func findHeader(v base.HeaderValue, prefix string) string {
	for _, vi := range v {
		if strings.HasPrefix(vi, prefix) {
			return vi
		}
	}
	return ""
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
func NewSender(v base.HeaderValue, user string, pass string) (*Sender, error) {
	// prefer digest
	if v0 := findHeader(v, "Digest"); v0 != "" {
		var auth headers.Authenticate
		err := auth.Unmarshal(base.HeaderValue{v0})
		if err != nil {
			return nil, err
		}

		return &Sender{
			user:               user,
			pass:               pass,
			authenticateHeader: &auth,
		}, nil
	}

	if v0 := findHeader(v, "Basic"); v0 != "" {
		var auth headers.Authenticate
		err := auth.Unmarshal(base.HeaderValue{v0})
		if err != nil {
			return nil, err
		}

		return &Sender{
			user:               user,
			pass:               pass,
			authenticateHeader: &auth,
		}, nil
	}

	return nil, fmt.Errorf("no authentication methods available")
}

// AddAuthorization adds the Authorization header to a Request.
func (se *Sender) AddAuthorization(req *base.Request) {
	urStr := req.URL.CloneWithoutCredentials().String()

	h := headers.Authorization{
		Method: se.authenticateHeader.Method,
	}

	if se.authenticateHeader.Method == headers.AuthBasic {
		h.BasicUser = se.user
		h.BasicPass = se.pass
	} else { // digest
		h.Username = se.user
		h.Realm = se.authenticateHeader.Realm
		h.Nonce = se.authenticateHeader.Nonce
		h.URI = urStr
		h.Response = md5Hex(md5Hex(se.user+":"+se.authenticateHeader.Realm+":"+se.pass) + ":" +
			se.authenticateHeader.Nonce + ":" + md5Hex(string(req.Method)+":"+urStr))
	}

	if req.Header == nil {
		req.Header = make(base.Header)
	}

	req.Header["Authorization"] = h.Marshal()
}
