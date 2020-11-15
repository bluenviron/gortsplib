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

// ReadAuth parses an Authenticate or a WWW-Authenticate header.
func ReadAuth(v base.HeaderValue) (*Auth, error) {
	if len(v) == 0 {
		return nil, fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return nil, fmt.Errorf("value provided multiple times (%v)", v)
	}

	ha := &Auth{}

	v0 := v[0]

	i := strings.IndexByte(v0, ' ')
	if i < 0 {
		return nil, fmt.Errorf("unable to find method (%s)", v0)
	}

	switch v0[:i] {
	case "Basic":
		ha.Method = AuthBasic

	case "Digest":
		ha.Method = AuthDigest

	default:
		return nil, fmt.Errorf("invalid method (%s)", v0[:i])
	}
	v0 = v0[i+1:]

	for len(v0) > 0 {
		i := strings.IndexByte(v0, '=')
		if i < 0 {
			return nil, fmt.Errorf("unable to find key (%s)", v0)
		}
		var key string
		key, v0 = v0[:i], v0[i+1:]

		var val string
		var err error
		val, v0, err = findValue(v0)
		if err != nil {
			return nil, err
		}

		switch key {
		case "username":
			ha.Username = &val

		case "realm":
			ha.Realm = &val

		case "nonce":
			ha.Nonce = &val

		case "uri":
			ha.URI = &val

		case "response":
			ha.Response = &val

		case "opaque":
			ha.Opaque = &val

		case "stale":
			ha.Stale = &val

		case "algorithm":
			ha.Algorithm = &val

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

	return ha, nil
}

// Write encodes an Authenticate or a WWW-Authenticate header.
func (ha *Auth) Write() base.HeaderValue {
	ret := ""

	switch ha.Method {
	case AuthBasic:
		ret += "Basic"

	case AuthDigest:
		ret += "Digest"
	}

	ret += " "

	var vals []string

	if ha.Username != nil {
		vals = append(vals, "username=\""+*ha.Username+"\"")
	}

	if ha.Realm != nil {
		vals = append(vals, "realm=\""+*ha.Realm+"\"")
	}

	if ha.Nonce != nil {
		vals = append(vals, "nonce=\""+*ha.Nonce+"\"")
	}

	if ha.URI != nil {
		vals = append(vals, "uri=\""+*ha.URI+"\"")
	}

	if ha.Response != nil {
		vals = append(vals, "response=\""+*ha.Response+"\"")
	}

	if ha.Opaque != nil {
		vals = append(vals, "opaque=\""+*ha.Opaque+"\"")
	}

	if ha.Stale != nil {
		vals = append(vals, "stale=\""+*ha.Stale+"\"")
	}

	if ha.Algorithm != nil {
		vals = append(vals, "algorithm=\""+*ha.Algorithm+"\"")
	}

	ret += strings.Join(vals, ", ")

	return base.HeaderValue{ret}
}
