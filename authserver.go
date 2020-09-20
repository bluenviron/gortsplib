package gortsplib

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// AuthServer is an object that helps a server to validate the credentials of
// a client.
type AuthServer struct {
	user    string
	pass    string
	methods []AuthMethod
	realm   string
	nonce   string
}

// NewAuthServer allocates an AuthServer.
// If methods is nil, the Basic and Digest methods are used.
func NewAuthServer(user string, pass string, methods []AuthMethod) *AuthServer {
	if methods == nil {
		methods = []AuthMethod{Basic, Digest}
	}

	nonceByts := make([]byte, 16)
	rand.Read(nonceByts)
	nonce := hex.EncodeToString(nonceByts)

	return &AuthServer{
		user:    user,
		pass:    pass,
		methods: methods,
		realm:   "IPCAM",
		nonce:   nonce,
	}
}

// GenerateHeader generates the WWW-Authenticate header needed by a client to log in.
func (as *AuthServer) GenerateHeader() HeaderValue {
	var ret HeaderValue
	for _, m := range as.methods {
		switch m {
		case Basic:
			ret = append(ret, (&HeaderAuth{
				Method: Basic,
				Realm:  &as.realm,
			}).Write()...)

		case Digest:
			ret = append(ret, (&HeaderAuth{
				Method: Digest,
				Realm:  &as.realm,
				Nonce:  &as.nonce,
			}).Write()...)
		}
	}
	return ret
}

// ValidateHeader validates the Authorization header sent by a client after receiving the
// WWW-Authenticate header.
func (as *AuthServer) ValidateHeader(v HeaderValue, method Method, ur *url.URL) error {
	if len(v) == 0 {
		return fmt.Errorf("authorization header not provided")
	}
	if len(v) > 1 {
		return fmt.Errorf("authorization header provided multiple times")
	}

	v0 := v[0]

	if strings.HasPrefix(v0, "Basic ") {
		inResponse := v0[len("Basic "):]

		response := base64.StdEncoding.EncodeToString([]byte(as.user + ":" + as.pass))

		if inResponse != response {
			return fmt.Errorf("wrong response")
		}

	} else if strings.HasPrefix(v0, "Digest ") {
		auth, err := ReadHeaderAuth(HeaderValue{v0})
		if err != nil {
			return err
		}

		if auth.Realm == nil {
			return fmt.Errorf("realm not provided")
		}

		if auth.Nonce == nil {
			return fmt.Errorf("nonce not provided")
		}

		if auth.Username == nil {
			return fmt.Errorf("username not provided")
		}

		if auth.URI == nil {
			return fmt.Errorf("uri not provided")
		}

		if auth.Response == nil {
			return fmt.Errorf("response not provided")
		}

		if *auth.Nonce != as.nonce {
			return fmt.Errorf("wrong nonce")
		}

		if *auth.Realm != as.realm {
			return fmt.Errorf("wrong realm")
		}

		if *auth.Username != as.user {
			return fmt.Errorf("wrong username")
		}

		uri := ur.String()

		if *auth.URI != uri {
			// VLC strips the subpath
			newUrl := *ur
			newUrl.Path = func() string {
				ret := newUrl.Path

				if n := strings.Index(ret[1:], "/"); n >= 0 {
					ret = ret[:n+2]
				}

				return ret
			}()
			uri = newUrl.String()

			if *auth.URI != uri {
				return fmt.Errorf("wrong url")
			}
		}

		response := md5Hex(md5Hex(as.user+":"+as.realm+":"+as.pass) +
			":" + as.nonce + ":" + md5Hex(string(method)+":"+uri))

		if *auth.Response != response {
			return fmt.Errorf("wrong response")
		}

	} else {
		return fmt.Errorf("unsupported authorization header")
	}

	return nil
}
