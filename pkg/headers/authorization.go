package headers

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aler9/gortsplib/pkg/base"
)

// Authorization is an Authorization header.
type Authorization struct {
	// authentication method
	Method AuthMethod

	// basic user
	BasicUser string

	// basic password
	BasicPass string

	// digest values
	DigestValues Authenticate
}

// Read decodes an Authorization header.
func (h *Authorization) Read(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	v0 := v[0]

	switch {
	case strings.HasPrefix(v0, "Basic "):
		h.Method = AuthBasic

		v0 = v0[len("Basic "):]

		tmp, err := base64.StdEncoding.DecodeString(v0)
		if err != nil {
			return fmt.Errorf("invalid value")
		}

		tmp2 := strings.Split(string(tmp), ":")
		if len(tmp2) != 2 {
			return fmt.Errorf("invalid value")
		}

		h.BasicUser, h.BasicPass = tmp2[0], tmp2[1]

	case strings.HasPrefix(v0, "Digest "):
		h.Method = AuthDigest

		var vals Authenticate
		err := vals.Read(base.HeaderValue{v0})
		if err != nil {
			return err
		}

		h.DigestValues = vals

	default:
		return fmt.Errorf("invalid authorization header")
	}

	return nil
}

// Write encodes an Authorization header.
func (h Authorization) Write() base.HeaderValue {
	switch h.Method {
	case AuthBasic:
		response := base64.StdEncoding.EncodeToString([]byte(h.BasicUser + ":" + h.BasicPass))

		return base.HeaderValue{"Basic " + response}

	default: // AuthDigest
		return h.DigestValues.Write()
	}
}
