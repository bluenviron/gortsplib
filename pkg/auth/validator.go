package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

// Validator allows a server to validate some credentials sent by a client.
type Validator struct {
	user    string
	pass    string
	methods []headers.AuthMethod
	realm   string
	nonce   string
}

// NewValidator allocates a Validator.
// If methods is nil, the Basic and Digest methods are used.
func NewValidator(user string, pass string, methods []headers.AuthMethod) *Validator {
	if methods == nil {
		methods = []headers.AuthMethod{headers.AuthBasic, headers.AuthDigest}
	}

	nonceByts := make([]byte, 16)
	rand.Read(nonceByts)
	nonce := hex.EncodeToString(nonceByts)

	return &Validator{
		user:    user,
		pass:    pass,
		methods: methods,
		realm:   "IPCAM",
		nonce:   nonce,
	}
}

// GenerateHeader generates the WWW-Authenticate header needed by a client to
// authenticate.
func (va *Validator) GenerateHeader() base.HeaderValue {
	var ret base.HeaderValue
	for _, m := range va.methods {
		switch m {
		case headers.AuthBasic:
			ret = append(ret, (&headers.Auth{
				Method: headers.AuthBasic,
				Realm:  &va.realm,
			}).Write()...)

		case headers.AuthDigest:
			ret = append(ret, headers.Auth{
				Method: headers.AuthDigest,
				Realm:  &va.realm,
				Nonce:  &va.nonce,
			}.Write()...)
		}
	}
	return ret
}

// ValidateHeader validates the Authorization header sent by a client after receiving the
// WWW-Authenticate header.
func (va *Validator) ValidateHeader(v base.HeaderValue, method base.Method, ur *base.URL) error {
	if len(v) == 0 {
		return fmt.Errorf("authorization header not provided")
	}
	if len(v) > 1 {
		return fmt.Errorf("authorization header provided multiple times")
	}

	v0 := v[0]

	if strings.HasPrefix(v0, "Basic ") {
		inResponse := v0[len("Basic "):]

		response := base64.StdEncoding.EncodeToString([]byte(va.user + ":" + va.pass))

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

		if *auth.Nonce != va.nonce {
			return fmt.Errorf("wrong nonce")
		}

		if *auth.Realm != va.realm {
			return fmt.Errorf("wrong realm")
		}

		if *auth.Username != va.user {
			return fmt.Errorf("wrong username")
		}

		uri := ur.String()

		if *auth.URI != uri {
			// VLC strips the control attribute; do another try without the control attribute
			ur = ur.Clone()
			ur.RemoveControlAttribute()
			uri = ur.String()

			if *auth.URI != uri {
				return fmt.Errorf("wrong url")
			}
		}

		response := md5Hex(md5Hex(va.user+":"+va.realm+":"+va.pass) +
			":" + va.nonce + ":" + md5Hex(string(method)+":"+uri))

		if *auth.Response != response {
			return fmt.Errorf("wrong response")
		}

	} else {
		return fmt.Errorf("unsupported authorization header")
	}

	return nil
}
