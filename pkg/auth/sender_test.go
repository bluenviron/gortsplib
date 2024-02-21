package auth

import (
	"testing"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
)

func FuzzSender(f *testing.F) {
	f.Add(`Invalid`)
	f.Add(`Digest`)
	f.Add(`Digest nonce=123`)
	f.Add(`Digest realm=123`)
	f.Add(`Basic`)
	f.Add(`Basic nonce=123`)

	f.Fuzz(func(t *testing.T, a string) {
		NewSender(base.HeaderValue{a}, "myuser", "mypass") //nolint:errcheck
	})
}
