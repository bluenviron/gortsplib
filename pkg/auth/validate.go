package auth

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
)

func md5Hex(in string) string {
	h := md5.New()
	h.Write([]byte(in))
	return hex.EncodeToString(h.Sum(nil))
}

// GenerateNonce generates a nonce that can be used in Validate().
func GenerateNonce() (string, error) {
	byts := make([]byte, 16)
	_, err := rand.Read(byts)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(byts), nil
}

// GenerateWWWAuthenticate generates a WWW-Authenticate header.
func GenerateWWWAuthenticate(methods []headers.AuthMethod, realm string, nonce string) base.HeaderValue {
	if methods == nil {
		methods = []headers.AuthMethod{headers.AuthBasic, headers.AuthDigest}
	}

	var ret base.HeaderValue
	for _, m := range methods {
		switch m {
		case headers.AuthBasic:
			ret = append(ret, (&headers.Authenticate{
				Method: headers.AuthBasic,
				Realm:  &realm,
			}).Marshal()...)

		case headers.AuthDigest:
			ret = append(ret, headers.Authenticate{
				Method: headers.AuthDigest,
				Realm:  &realm,
				Nonce:  &nonce,
			}.Marshal()...)
		}
	}
	return ret
}

func contains(list []headers.AuthMethod, item headers.AuthMethod) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}

// Validate validates a request sent by a client.
func Validate(
	req *base.Request,
	user string,
	pass string,
	baseURL *base.URL,
	methods []headers.AuthMethod,
	realm string,
	nonce string,
) error {
	if methods == nil {
		methods = []headers.AuthMethod{headers.AuthBasic, headers.AuthDigest}
	}

	var auth headers.Authorization
	err := auth.Unmarshal(req.Header["Authorization"])
	if err != nil {
		return err
	}

	switch {
	case auth.Method == headers.AuthBasic && contains(methods, headers.AuthBasic):
		if auth.BasicUser != user {
			return fmt.Errorf("authentication failed")
		}

		if auth.BasicPass != pass {
			return fmt.Errorf("authentication failed")
		}
	case auth.Method == headers.AuthDigest && contains(methods, headers.AuthDigest):
		if auth.DigestValues.Realm == nil {
			return fmt.Errorf("realm is missing")
		}

		if auth.DigestValues.Nonce == nil {
			return fmt.Errorf("nonce is missing")
		}

		if auth.DigestValues.Username == nil {
			return fmt.Errorf("username is missing")
		}

		if auth.DigestValues.URI == nil {
			return fmt.Errorf("uri is missing")
		}

		if auth.DigestValues.Response == nil {
			return fmt.Errorf("response is missing")
		}

		if *auth.DigestValues.Nonce != nonce {
			return fmt.Errorf("wrong nonce")
		}

		if *auth.DigestValues.Realm != realm {
			return fmt.Errorf("wrong realm")
		}

		if *auth.DigestValues.Username != user {
			return fmt.Errorf("authentication failed")
		}

		ur := req.URL

		if *auth.DigestValues.URI != ur.String() {
			// in SETUP requests, VLC strips the control attribute.
			// try again with the base URL.
			if baseURL != nil {
				ur = baseURL
				if *auth.DigestValues.URI != ur.String() {
					return fmt.Errorf("wrong URL")
				}
			} else {
				return fmt.Errorf("wrong URL")
			}
		}

		response := md5Hex(md5Hex(user+":"+realm+":"+pass) +
			":" + nonce + ":" + md5Hex(string(req.Method)+":"+ur.String()))

		if *auth.DigestValues.Response != response {
			return fmt.Errorf("authentication failed")
		}
	default:
		return fmt.Errorf("no supported authentication methods found")
	}

	return nil
}
