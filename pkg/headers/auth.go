// Package headers contains various RTSP headers.
package headers

import (
	"fmt"
	"strings"

	"github.com/aler9/gortsplib/pkg/base"
)

// AuthMethod is an authentication method.
type AuthMethod int

const (
	// AuthBasic is the Basic authentication method
	AuthBasic AuthMethod = iota

	// AuthDigest is the Digest authentication method
	AuthDigest
)

// Auth is an Authenticate or a WWWW-Authenticate header.
type Auth struct {
	// authentication method
	Method AuthMethod

	// (optional) username
	Username *string

	// (optional) realm
	Realm *string

	// (optional) nonce
	Nonce *string

	// (optional) uri
	URI *string

	// (optional) response
	Response *string

	// (optional) opaque
	Opaque *string

	// (optional) stale
	Stale *string

	// (optional) algorithm
	Algorithm *string
}

func findValue(v0 string) (string, string, error) {
	if v0 == "" {
		return "", "", nil
	}

	if v0[0] == '"' {
		i := 1
		for {
			if i >= len(v0) {
				return "", "", fmt.Errorf("apices not closed (%v)", v0)
			}

			if v0[i] == '"' {
				return v0[1:i], v0[i+1:], nil
			}

			i++
		}
	}

	i := 0
	for {
		if i >= len(v0) || v0[i] == ',' {
			return v0[:i], v0[i:], nil
		}

		i++
	}
}

// Read decodes an Authenticate or a WWW-Authenticate header.
func (h *Auth) Read(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	v0 := v[0]

	i := strings.IndexByte(v0, ' ')
	if i < 0 {
		return fmt.Errorf("unable to find method (%s)", v0)
	}

	switch v0[:i] {
	case "Basic":
		h.Method = AuthBasic

	case "Digest":
		h.Method = AuthDigest

	default:
		return fmt.Errorf("invalid method (%s)", v0[:i])
	}
	v0 = v0[i+1:]

	for len(v0) > 0 {
		i := strings.IndexByte(v0, '=')
		if i < 0 {
			return fmt.Errorf("unable to find key (%s)", v0)
		}
		var key string
		key, v0 = v0[:i], v0[i+1:]

		var val string
		var err error
		val, v0, err = findValue(v0)
		if err != nil {
			return err
		}

		switch key {
		case "username":
			h.Username = &val

		case "realm":
			h.Realm = &val

		case "nonce":
			h.Nonce = &val

		case "uri":
			h.URI = &val

		case "response":
			h.Response = &val

		case "opaque":
			h.Opaque = &val

		case "stale":
			h.Stale = &val

		case "algorithm":
			h.Algorithm = &val

			// ignore non-standard keys
		}

		// skip comma
		if len(v0) > 0 && v0[0] == ',' {
			v0 = v0[1:]
		}

		// skip spaces
		for len(v0) > 0 && v0[0] == ' ' {
			v0 = v0[1:]
		}
	}

	return nil
}

// Write encodes an Authenticate or a WWW-Authenticate header.
func (h Auth) Write() base.HeaderValue {
	ret := ""

	switch h.Method {
	case AuthBasic:
		ret += "Basic"

	case AuthDigest:
		ret += "Digest"
	}

	ret += " "

	var rets []string

	if h.Username != nil {
		rets = append(rets, "username=\""+*h.Username+"\"")
	}

	if h.Realm != nil {
		rets = append(rets, "realm=\""+*h.Realm+"\"")
	}

	if h.Nonce != nil {
		rets = append(rets, "nonce=\""+*h.Nonce+"\"")
	}

	if h.URI != nil {
		rets = append(rets, "uri=\""+*h.URI+"\"")
	}

	if h.Response != nil {
		rets = append(rets, "response=\""+*h.Response+"\"")
	}

	if h.Opaque != nil {
		rets = append(rets, "opaque=\""+*h.Opaque+"\"")
	}

	if h.Stale != nil {
		rets = append(rets, "stale=\""+*h.Stale+"\"")
	}

	if h.Algorithm != nil {
		rets = append(rets, "algorithm=\""+*h.Algorithm+"\"")
	}

	ret += strings.Join(rets, ", ")

	return base.HeaderValue{ret}
}
