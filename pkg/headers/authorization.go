package headers

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
)

// Authorization is an Authorization header.
type Authorization struct {
	// authentication method
	Method AuthMethod

	//
	// Basic authentication fields
	//

	// user
	BasicUser string

	// password
	BasicPass string

	//
	// Digest authentication fields
	//

	// username
	Username string

	// realm
	Realm string

	// nonce
	Nonce string

	// URI
	URI string

	// response
	Response string

	// opaque
	Opaque *string
}

// Unmarshal decodes an Authorization header.
func (h *Authorization) Unmarshal(v base.HeaderValue) error {
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
		tmp, err := base64.StdEncoding.DecodeString(v0)
		if err != nil {
			return fmt.Errorf("invalid value")
		}

		tmp2 := strings.Split(string(tmp), ":")
		if len(tmp2) != 2 {
			return fmt.Errorf("invalid value")
		}

		h.BasicUser, h.BasicPass = tmp2[0], tmp2[1]
	} else { // digest
		kvs, err := keyValParse(v0, ',')
		if err != nil {
			return err
		}

		realmReceived := false
		usernameReceived := false
		nonceReceived := false
		uriReceived := false
		responseReceived := false
		var algorithm *string

		for k, rv := range kvs {
			v := rv

			switch k {
			case "realm":
				h.Realm = v
				realmReceived = true

			case "username":
				h.Username = v
				usernameReceived = true

			case "nonce":
				h.Nonce = v
				nonceReceived = true

			case "uri":
				h.URI = v
				uriReceived = true

			case "response":
				h.Response = v
				responseReceived = true

			case "opaque":
				h.Opaque = &v

			case "algorithm":
				algorithm = &v
			}
		}

		if !realmReceived || !usernameReceived || !nonceReceived || !uriReceived || !responseReceived {
			return fmt.Errorf("one or more digest fields are missing")
		}

		h.Method, err = algorithmToMethod(algorithm)
		if err != nil {
			return err
		}
	}

	return nil
}

// Marshal encodes an Authorization header.
func (h Authorization) Marshal() base.HeaderValue {
	if h.Method == AuthBasic {
		return base.HeaderValue{"Basic " +
			base64.StdEncoding.EncodeToString([]byte(h.BasicUser+":"+h.BasicPass))}
	}

	ret := "Digest " +
		"username=\"" + h.Username + "\", realm=\"" + h.Realm + "\", " +
		"nonce=\"" + h.Nonce + "\", uri=\"" + h.URI + "\", response=\"" + h.Response + "\""

	if h.Opaque != nil {
		ret += ", opaque=\"" + *h.Opaque + "\""
	}

	if h.Method == AuthDigestMD5 {
		ret += ", algorithm=\"MD5\""
	} else {
		ret += ", algorithm=\"SHA-256\""
	}

	return base.HeaderValue{ret}
}
