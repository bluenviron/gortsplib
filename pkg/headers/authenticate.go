// Package headers contains various RTSP headers.
package headers

import (
	"fmt"
	"strings"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
)

// AuthMethod is an authentication method.
type AuthMethod int

const (
	// AuthBasic is the Basic authentication method
	AuthBasic AuthMethod = iota

	// AuthDigestMD5 is the Digest authentication method with the MD5 hash
	AuthDigestMD5

	// AuthDigestSHA256 is the Digest authentication method with the SHA-256 hash
	AuthDigestSHA256
)

const (
	// AuthDigest is an alias for AuthDigestMD5
	//
	// Deprecated: replaced by AuthDigestMD5
	AuthDigest = AuthDigestMD5
)

func algorithmToMethod(v *string) (AuthMethod, error) {
	switch {
	case v == nil, strings.ToLower(*v) == "md5":
		return AuthDigestMD5, nil

	case strings.ToLower(*v) == "sha-256":
		return AuthDigestSHA256, nil

	default:
		return 0, fmt.Errorf("unrecognized algorithm: %v", *v)
	}
}

// Authenticate is a WWW-Authenticate header.
type Authenticate struct {
	// authentication method
	Method AuthMethod

	// realm
	Realm string

	//
	// Digest authentication fields
	//

	// nonce
	Nonce string

	// opaque
	Opaque *string

	// stale
	Stale *string
}

// Unmarshal decodes a WWW-Authenticate header.
func (h *Authenticate) Unmarshal(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	v0 := v[0]

	i := strings.IndexByte(v0, ' ')
	if i < 0 {
		return fmt.Errorf("unable to split between method and keys (%v)", v0)
	}
	method, v0 := v0[:i], v0[i+1:]

	isDigest := false

	switch method {
	case "Basic":
		h.Method = AuthBasic

	case "Digest":
		isDigest = true

	default:
		return fmt.Errorf("invalid method (%s)", method)
	}

	if !isDigest {
		kvs, err := keyValParse(v0, ',')
		if err != nil {
			return err
		}

		realmReceived := false

		for k, rv := range kvs {
			v := rv

			if k == "realm" {
				h.Realm = v
				realmReceived = true
			}
		}

		if !realmReceived {
			return fmt.Errorf("realm is missing")
		}
	} else { // digest
		kvs, err := keyValParse(v0, ',')
		if err != nil {
			return err
		}

		realmReceived := false
		nonceReceived := false
		var algorithm *string

		for k, rv := range kvs {
			v := rv

			switch k {
			case "realm":
				h.Realm = v
				realmReceived = true

			case "nonce":
				h.Nonce = v
				nonceReceived = true

			case "opaque":
				h.Opaque = &v

			case "stale":
				h.Stale = &v

			case "algorithm":
				algorithm = &v
			}
		}

		if !realmReceived || !nonceReceived {
			return fmt.Errorf("one or more digest fields are missing")
		}

		h.Method, err = algorithmToMethod(algorithm)
		if err != nil {
			return err
		}
	}

	return nil
}

// Marshal encodes a WWW-Authenticate header.
func (h Authenticate) Marshal() base.HeaderValue {
	if h.Method == AuthBasic {
		return base.HeaderValue{"Basic " +
			"realm=\"" + h.Realm + "\""}
	}

	ret := "Digest realm=\"" + h.Realm + "\", nonce=\"" + h.Nonce + "\""

	if h.Opaque != nil {
		ret += ", opaque=\"" + *h.Opaque + "\""
	}

	if h.Stale != nil {
		ret += ", stale=\"" + *h.Stale + "\""
	}

	if h.Method == AuthDigestMD5 {
		ret += ", algorithm=\"MD5\""
	} else {
		ret += ", algorithm=\"SHA-256\""
	}

	return base.HeaderValue{ret}
}
