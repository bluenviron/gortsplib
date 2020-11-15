package auth

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

// Client is an object that allows a client to authenticate against a server.
type Client struct {
	user   string
	pass   string
	method headers.AuthMethod
	realm  string
	nonce  string
}

// NewClient allocates a Client with the WWW-Authenticate header provided by
// the server and a set of credentials.
func NewClient(v base.HeaderValue, userinfo *url.Userinfo) (*Client, error) {
	pass, _ := userinfo.Password()
	user := userinfo.Username()

	// prefer digest
	if headerAuthDigest := func() string {
		for _, vi := range v {
			if strings.HasPrefix(vi, "Digest ") {
				return vi
			}
		}
		return ""
	}(); headerAuthDigest != "" {
		auth, err := headers.ReadAuth(base.HeaderValue{headerAuthDigest})
		if err != nil {
			return nil, err
		}

		if auth.Realm == nil {
			return nil, fmt.Errorf("realm not provided")
		}

		if auth.Nonce == nil {
			return nil, fmt.Errorf("nonce not provided")
		}

		return &Client{
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
		auth, err := headers.ReadAuth(base.HeaderValue{headerAuthBasic})
		if err != nil {
			return nil, err
		}

		if auth.Realm == nil {
			return nil, fmt.Errorf("realm not provided")
		}

		return &Client{
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
func (ac *Client) GenerateHeader(method base.Method, ur *base.URL) base.HeaderValue {
	urStr := ur.CloneWithoutCredentials().String()

	switch ac.method {
	case headers.AuthBasic:
		response := base64.StdEncoding.EncodeToString([]byte(ac.user + ":" + ac.pass))

		return base.HeaderValue{"Basic " + response}

	case headers.AuthDigest:
		response := md5Hex(md5Hex(ac.user+":"+ac.realm+":"+ac.pass) + ":" +
			ac.nonce + ":" + md5Hex(string(method)+":"+urStr))

		return (&headers.Auth{
			Method:   headers.AuthDigest,
			Username: &ac.user,
			Realm:    &ac.realm,
			Nonce:    &ac.nonce,
			URI:      &urStr,
			Response: &response,
		}).Write()
	}

	return nil
}
