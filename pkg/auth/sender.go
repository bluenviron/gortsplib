package auth

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

// Sender allows to generate credentials for a Validator.
type Sender struct {
	user   string
	pass   string
	method headers.AuthMethod
	realm  string
	nonce  string
}

// NewSender allocates a Sender with the WWW-Authenticate header provided by
// a Validator and a set of credentials.
func NewSender(v base.HeaderValue, user string, pass string) (*Sender, error) {
	// prefer digest
	if headerAuthDigest := func() string {
		for _, vi := range v {
			if strings.HasPrefix(vi, "Digest ") {
				return vi
			}
		}
		return ""
	}(); headerAuthDigest != "" {
		var auth headers.Auth
		err := auth.Read(base.HeaderValue{headerAuthDigest})
		if err != nil {
			return nil, err
		}

		if auth.Realm == nil {
			return nil, fmt.Errorf("realm not provided")
		}

		if auth.Nonce == nil {
			return nil, fmt.Errorf("nonce not provided")
		}

		return &Sender{
			user:   user,
			pass:   pass,
			method: headers.AuthDigest,
			realm:  *auth.Realm,
			nonce:  *auth.Nonce,
		}, nil
	}

	if headerAuthBasic := func() string {
		for _, vi := range v {
			if strings.HasPrefix(vi, "Basic ") {
				return vi
			}
		}
		return ""
	}(); headerAuthBasic != "" {
		var auth headers.Auth
		err := auth.Read(base.HeaderValue{headerAuthBasic})
		if err != nil {
			return nil, err
		}

		if auth.Realm == nil {
			return nil, fmt.Errorf("realm not provided")
		}

		return &Sender{
			user:   user,
			pass:   pass,
			method: headers.AuthBasic,
			realm:  *auth.Realm,
		}, nil
	}

	return nil, fmt.Errorf("there are no authentication methods available")
}

// GenerateHeader generates an Authorization Header that allows to authenticate a request with
// the given method and url.
func (se *Sender) GenerateHeader(method base.Method, ur *base.URL) base.HeaderValue {
	urStr := ur.CloneWithoutCredentials().String()

	switch se.method {
	case headers.AuthBasic:
		response := base64.StdEncoding.EncodeToString([]byte(se.user + ":" + se.pass))

		return base.HeaderValue{"Basic " + response}

	case headers.AuthDigest:
		response := md5Hex(md5Hex(se.user+":"+se.realm+":"+se.pass) + ":" +
			se.nonce + ":" + md5Hex(string(method)+":"+urStr))

		return headers.Auth{
			Method:   headers.AuthDigest,
			Username: &se.user,
			Realm:    &se.realm,
			Nonce:    &se.nonce,
			URI:      &urStr,
			Response: &response,
		}.Write()
	}

	return nil
}
