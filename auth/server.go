package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/aler9/gortsplib/base"
	"github.com/aler9/gortsplib/headers"
)

// Server allows a server to authenticate a client.
type Server struct {
	user    string
	pass    string
	methods []headers.AuthMethod
	realm   string
	nonce   string
}

// NewServer allocates a Server.
// If methods is nil, the Basic and Digest methods are used.
func NewServer(user string, pass string, methods []headers.AuthMethod) *Server {
	if methods == nil {
		methods = []headers.AuthMethod{headers.AuthBasic, headers.AuthDigest}
	}

	nonceByts := make([]byte, 16)
	rand.Read(nonceByts)
	nonce := hex.EncodeToString(nonceByts)

	return &Server{
		user:    user,
		pass:    pass,
		methods: methods,
		realm:   "IPCAM",
		nonce:   nonce,
	}
}

// GenerateHeader generates the WWW-Authenticate header needed by a client to
// authenticate.
func (as *Server) GenerateHeader() base.HeaderValue {
	var ret base.HeaderValue
	for _, m := range as.methods {
		switch m {
		case headers.AuthBasic:
			ret = append(ret, (&headers.Auth{
				Method: headers.AuthBasic,
				Realm:  &as.realm,
			}).Write()...)

		case headers.AuthDigest:
			ret = append(ret, (&headers.Auth{
				Method: headers.AuthDigest,
				Realm:  &as.realm,
				Nonce:  &as.nonce,
			}).Write()...)
		}
	}
	return ret
}

// ValidateHeader validates the Authorization header sent by a client after receiving the
// WWW-Authenticate header.
func (as *Server) ValidateHeader(v base.HeaderValue, method base.Method, ur *url.URL) error {
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
		auth, err := headers.ReadAuth(base.HeaderValue{v0})
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
			// VLC strips the control path; do another try without the control path
			base, _, ok := base.URLGetBaseControlPath(ur)
			if ok {
				ur.Path = "/" + base + "/"
				uri = ur.String()
			}

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
