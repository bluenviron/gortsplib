package auth

import (
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/headers"
)

var reControlAttribute = regexp.MustCompile("^(.+/)trackID=[0-9]+$")

func md5Hex(in string) string {
	h := md5.New()
	h.Write([]byte(in))
	return hex.EncodeToString(h.Sum(nil))
}

func sha256Hex(in string) string {
	h := sha256.New()
	h.Write([]byte(in))
	return hex.EncodeToString(h.Sum(nil))
}

func contains(list []ValidateMethod, item ValidateMethod) bool {
	for _, i := range list {
		if i == item {
			return true
		}
	}
	return false
}

func urlMatches(expected string, received string, isSetup bool) bool {
	if received == expected {
		return true
	}

	// in SETUP requests, VLC uses the base URL of the stream
	// instead of the URL of the track.
	// Strip the control attribute to obtain the URL of the stream.
	if isSetup {
		if m := reControlAttribute.FindStringSubmatch(expected); m != nil && received == m[1] {
			return true
		}
	}

	return false
}

// ValidateMethod is a validation method.
type ValidateMethod int

// validation methods.
const (
	ValidateMethodBasic ValidateMethod = iota
	ValidateMethodDigestMD5
	ValidateMethodSHA256
)

// Validate validates a request sent by a client.
func Validate(
	req *base.Request,
	user string,
	pass string,
	methods []ValidateMethod,
	realm string,
	nonce string,
) error {
	if methods == nil {
		methods = []ValidateMethod{ValidateMethodBasic, ValidateMethodDigestMD5, ValidateMethodSHA256}
	}

	var auth headers.Authorization
	err := auth.Unmarshal(req.Header["Authorization"])
	if err != nil {
		return err
	}

	switch {
	case auth.Method == headers.AuthMethodDigest &&
		(contains(methods, ValidateMethodDigestMD5) &&
			(auth.Algorithm == nil || *auth.Algorithm == headers.AuthAlgorithmMD5) ||
			contains(methods, ValidateMethodSHA256) &&
				auth.Algorithm != nil && *auth.Algorithm == headers.AuthAlgorithmSHA256):
		if auth.Nonce != nonce {
			return fmt.Errorf("wrong nonce")
		}

		if auth.Realm != realm {
			return fmt.Errorf("wrong realm")
		}

		if auth.Username != user {
			return fmt.Errorf("authentication failed")
		}

		if !urlMatches(req.URL.String(), auth.URI, req.Method == base.Setup) {
			return fmt.Errorf("wrong URL")
		}

		var response string

		if auth.Algorithm == nil || *auth.Algorithm == headers.AuthAlgorithmMD5 {
			response = md5Hex(md5Hex(user+":"+realm+":"+pass) +
				":" + nonce + ":" + md5Hex(string(req.Method)+":"+auth.URI))
		} else { // sha256
			response = sha256Hex(sha256Hex(user+":"+realm+":"+pass) +
				":" + nonce + ":" + sha256Hex(string(req.Method)+":"+auth.URI))
		}

		if auth.Response != response {
			return fmt.Errorf("authentication failed")
		}

	case auth.Method == headers.AuthMethodBasic && contains(methods, ValidateMethodBasic):
		if auth.BasicUser != user {
			return fmt.Errorf("authentication failed")
		}

		if auth.BasicPass != pass {
			return fmt.Errorf("authentication failed")
		}

	default:
		return fmt.Errorf("no supported authentication methods found")
	}

	return nil
}
