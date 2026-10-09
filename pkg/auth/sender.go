package auth

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/headers"
)

func hasQOPAuth(qop *string) bool {
	if qop != nil {
		for v := range strings.SplitSeq(*qop, ",") {
			if strings.TrimSpace(v) == "auth" {
				return true
			}
		}
	}
	return false
}

// SenderPolicy restricts the authentication methods that a Sender can use.
// The zero value allows all methods.
type SenderPolicy struct {
	// Do not answer Digest challenges that use MD5.
	// MD5 is not an approved algorithm in FIPS 140 environments.
	RefuseMD5 bool

	// Do not send Basic credentials over connections that are not encrypted,
	// since they would travel in clear text.
	RefuseCleartextBasic bool
}

// Sender allows to send credentials.
// It requires a WWW-Authenticate header (provided by the server)
// and a set of credentials.
type Sender struct {
	WWWAuth base.HeaderValue
	User    string
	Pass    string

	// Restrictions on the authentication method (optional).
	Policy SenderPolicy

	// Whether the connection is encrypted (TLS).
	// Used with Policy.RefuseCleartextBasic.
	Encrypted bool

	authHeader *headers.Authenticate
	hasQOPAuth bool
	cnonce     string
	nonceCount uint32
}

// Initialize initializes a Sender.
func (se *Sender) Initialize() error {
	var refused []string

	for _, v := range se.WWWAuth {
		var auth headers.Authenticate
		err := auth.Unmarshal(base.HeaderValue{v})
		if err != nil {
			continue // ignore unrecognized headers
		}

		if reason := se.Policy.refuses(&auth, se.Encrypted); reason != "" {
			refused = append(refused, reason)
			continue
		}

		if se.authHeader == nil ||
			(auth.Algorithm != nil && *auth.Algorithm == headers.AuthAlgorithmSHA256) ||
			(se.authHeader.Method == headers.AuthMethodBasic) {
			se.authHeader = &auth
		}
	}

	if se.authHeader == nil {
		if refused != nil {
			return fmt.Errorf("no authentication methods allowed by policy (refused: %s)",
				strings.Join(refused, ", "))
		}
		return fmt.Errorf("no authentication methods available")
	}

	if hasQOPAuth(se.authHeader.Qop) {
		se.hasQOPAuth = true

		var err error
		se.cnonce, err = GenerateNonce()
		if err != nil {
			return err
		}
	}

	return nil
}

// AddAuthorization adds the Authorization header to a Request.
func (se *Sender) AddAuthorization(req *base.Request) {
	urStr := req.URL.CloneWithoutCredentials().String()

	h := headers.Authorization{
		Method: se.authHeader.Method,
	}

	h.Username = se.User

	if se.authHeader.Method == headers.AuthMethodBasic {
		h.BasicPass = se.Pass
	} else { // digest
		h.Realm = se.authHeader.Realm
		h.Nonce = se.authHeader.Nonce
		h.URI = urStr
		h.Algorithm = se.authHeader.Algorithm
		h.Opaque = se.authHeader.Opaque

		middle := se.authHeader.Nonce

		if se.hasQOPAuth {
			// the nonce count must increase at every request that uses the same nonce.
			se.nonceCount++
			nc := strconv.FormatUint(uint64(se.nonceCount), 16)
			nc = strings.Repeat("0", 8-len(nc)) + nc

			qop := "auth"
			h.Qop = &qop
			h.Cnonce = &se.cnonce
			h.Nc = &nc

			middle += ":" + nc + ":" + se.cnonce + ":" + qop
		}

		if se.authHeader.Algorithm == nil || *se.authHeader.Algorithm == headers.AuthAlgorithmMD5 {
			h.Response = md5Hex(md5Hex(se.User+":"+se.authHeader.Realm+":"+se.Pass) + ":" +
				middle + ":" + md5Hex(string(req.Method)+":"+urStr))
		} else { // sha256
			h.Response = sha256Hex(sha256Hex(se.User+":"+se.authHeader.Realm+":"+se.Pass) + ":" +
				middle + ":" + sha256Hex(string(req.Method)+":"+urStr))
		}
	}

	if req.Header == nil {
		req.Header = make(base.Header)
	}

	req.Header["Authorization"] = h.Marshal()
}

func (p SenderPolicy) refuses(auth *headers.Authenticate, encrypted bool) string {
	switch {
	case p.RefuseCleartextBasic && auth.Method == headers.AuthMethodBasic && !encrypted:
		return "Basic without TLS"

	case p.RefuseMD5 && auth.Method == headers.AuthMethodDigest &&
		(auth.Algorithm == nil || *auth.Algorithm == headers.AuthAlgorithmMD5):
		return "Digest MD5"
	}
	return ""
}
