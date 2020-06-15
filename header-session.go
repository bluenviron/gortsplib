package gortsplib

import (
	"fmt"
	"strconv"
	"strings"
)

// HeaderSession is a Session header.
type HeaderSession struct {
	Session string
	Timeout *uint
}

// ReadHeaderSession parses a Session header.
func ReadHeaderSession(in string) (*HeaderSession, error) {
	parts := strings.Split(in, ";")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid value")
	}

	hs := &HeaderSession{}

	hs.Session, parts = parts[0], parts[1:]

	for _, part := range parts {
		// remove leading spaces
		part = strings.TrimLeft(part, " ")

		keyval := strings.Split(part, "=")
		if len(keyval) != 2 {
			return nil, fmt.Errorf("invalid value")
		}

		key, strValue := keyval[0], keyval[1]
		if key != "timeout" {
			return nil, fmt.Errorf("invalid key '%s'", key)
		}

		iv, err := strconv.ParseUint(strValue, 10, 64)
		if err != nil {
			return nil, err
		}
		uiv := uint(iv)

		hs.Timeout = &uiv
	}

	return hs, nil
}
