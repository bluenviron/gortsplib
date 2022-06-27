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

// Authenticate is an Authenticate or a WWW-Authenticate header.
type Authenticate struct {
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

// Unmarshal decodes an Authenticate or a WWW-Authenticate header.
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

	switch method {
	case "Basic":
		h.Method = AuthBasic

	case "Digest":
		h.Method = AuthDigest

	default:
		return fmt.Errorf("invalid method (%s)", method)
	}

	kvs, err := keyValParse(v0, ',')
	if err != nil {
		return err
	}

	for k, rv := range kvs {
		v := rv

		switch k {
		case "username":
			h.Username = &v

		case "realm":
			h.Realm = &v

		case "nonce":
			h.Nonce = &v

		case "uri":
			h.URI = &v

		case "response":
			h.Response = &v

		case "opaque":
			h.Opaque = &v

		case "stale":
			h.Stale = &v

		case "algorithm":
			h.Algorithm = &v
		}
	}

	return nil
}

// Marshal encodes an Authenticate or a WWW-Authenticate header.
func (h Authenticate) Marshal() base.HeaderValue {
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
