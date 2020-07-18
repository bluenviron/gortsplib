package gortsplib

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

func md5Hex(in string) string {
	h := md5.New()
	h.Write([]byte(in))
	return hex.EncodeToString(h.Sum(nil))
}

// AuthMethod is an authentication method.
type AuthMethod int

const (
	Basic AuthMethod = iota
	Digest
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
// If methods is nil, Basic and Digest authentications are used.
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
				Prefix: "Basic",
				Values: map[string]string{
					"realm": as.realm,
				},
			}).Write()...)

		case Digest:
			ret = append(ret, (&HeaderAuth{
				Prefix: "Digest",
				Values: map[string]string{
					"realm": as.realm,
					"nonce": as.nonce,
				},
			}).Write()...)
		}
	}
	return ret
}

// ValidateHeader validates the Authorization header sent by a client after receiving the
// WWW-Authenticate header provided by GenerateHeader().
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

		inRealm, ok := auth.Values["realm"]
		if !ok {
			return fmt.Errorf("realm not provided")
		}

		inNonce, ok := auth.Values["nonce"]
		if !ok {
			return fmt.Errorf("nonce not provided")
		}

		inUsername, ok := auth.Values["username"]
		if !ok {
			return fmt.Errorf("username not provided")
		}

		inUri, ok := auth.Values["uri"]
		if !ok {
			return fmt.Errorf("uri not provided")
		}

		inResponse, ok := auth.Values["response"]
		if !ok {
			return fmt.Errorf("response not provided")
		}

		if inNonce != as.nonce {
			return fmt.Errorf("wrong nonce")
		}

		if inRealm != as.realm {
			return fmt.Errorf("wrong realm")
		}

		if inUsername != as.user {
			return fmt.Errorf("wrong username")
		}

		uri := ur.String()

		if inUri != uri {
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

			if inUri != uri {
				return fmt.Errorf("wrong url")
			}
		}

		response := md5Hex(md5Hex(as.user+":"+as.realm+":"+as.pass) +
			":" + as.nonce + ":" + md5Hex(string(method)+":"+uri))

		if inResponse != response {
			return fmt.Errorf("wrong response")
		}

	} else {
		return fmt.Errorf("unsupported authorization header")
	}

	return nil
}

// authClient is an object that helps a client to send its credentials to a
// server.
type authClient struct {
	user   string
	pass   string
	method AuthMethod
	realm  string
	nonce  string
}

// newAuthClient allocates an authClient.
// header is the WWW-Authenticate header provided by the server.
func newAuthClient(v HeaderValue, user string, pass string) (*authClient, error) {
	// prefer digest
	if headerAuthDigest := func() string {
		for _, vi := range v {
			if strings.HasPrefix(vi, "Digest ") {
				return vi
			}
		}
		return ""
	}(); headerAuthDigest != "" {
		auth, err := ReadHeaderAuth(HeaderValue{headerAuthDigest})
		if err != nil {
			return nil, err
		}

		realm, ok := auth.Values["realm"]
		if !ok {
			return nil, fmt.Errorf("realm not provided")
		}

		nonce, ok := auth.Values["nonce"]
		if !ok {
			return nil, fmt.Errorf("nonce not provided")
		}

		return &authClient{
			user:   user,
			pass:   pass,
			method: Digest,
			realm:  realm,
			nonce:  nonce,
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
		auth, err := ReadHeaderAuth(HeaderValue{headerAuthBasic})
		if err != nil {
			return nil, err
		}

		realm, ok := auth.Values["realm"]
		if !ok {
			return nil, fmt.Errorf("realm not provided")
		}

		return &authClient{
			user:   user,
			pass:   pass,
			method: Basic,
			realm:  realm,
		}, nil
	}

	return nil, fmt.Errorf("there are no authentication methods available")
}

// GenerateHeader generates an Authorization Header that allows to authenticate a request with
// the given method and url.
func (ac *authClient) GenerateHeader(method Method, ur *url.URL) HeaderValue {
	switch ac.method {
	case Basic:
		response := base64.StdEncoding.EncodeToString([]byte(ac.user + ":" + ac.pass))

		return HeaderValue{"Basic " + response}

	case Digest:
		response := md5Hex(md5Hex(ac.user+":"+ac.realm+":"+ac.pass) + ":" +
			ac.nonce + ":" + md5Hex(string(method)+":"+ur.String()))

		return (&HeaderAuth{
			Prefix: "Digest",
			Values: map[string]string{
				"username": ac.user,
				"realm":    ac.realm,
				"nonce":    ac.nonce,
				"uri":      ur.String(),
				"response": response,
			},
		}).Write()
	}

	return nil
}
