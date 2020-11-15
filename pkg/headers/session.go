package headers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aler9/gortsplib/pkg/base"
)

// Session is a Session header.
type Session struct {
	// session id
	Session string

	// (optional) a timeout
	Timeout *uint
}

// ReadSession parses a Session header.
func ReadSession(v base.HeaderValue) (*Session, error) {
	if len(v) == 0 {
		return nil, fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return nil, fmt.Errorf("value provided multiple times (%v)", v)
	}

	parts := strings.Split(v[0], ";")
	if len(parts) == 0 {
		return nil, fmt.Errorf("invalid value (%v)", v)
	}

	hs := &Session{}

	hs.Session = parts[0]

	for _, part := range parts[1:] {
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

// Write encodes a Session header
func (hs *Session) Write() base.HeaderValue {
	val := hs.Session

	if hs.Timeout != nil {
		val += ";timeout=" + strconv.FormatUint(uint64(*hs.Timeout), 10)
	}

	return base.HeaderValue{val}
}
