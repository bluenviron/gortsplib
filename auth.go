package gortsplib

import (
	"crypto/md5"
	"encoding/hex"
)

func md5Hex(in string) string {
	h := md5.New()
	h.Write([]byte(in))
	return hex.EncodeToString(h.Sum(nil))
}

// AuthMethod is an authentication method.
type AuthMethod int

const (
	// Basic authentication method
	Basic AuthMethod = iota

	// Digest authentication method
	Digest
)
