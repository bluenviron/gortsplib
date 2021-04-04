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

// Read decodes a Session header.
func (h *Session) Read(v base.HeaderValue) error {
	if len(v) == 0 {
		return fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return fmt.Errorf("value provided multiple times (%v)", v)
	}

	parts := strings.Split(v[0], ";")
	if len(parts) == 0 {
		return fmt.Errorf("invalid value (%v)", v)
	}

	h.Session = parts[0]

	for _, kv := range parts[1:] {
		// remove leading spaces
		kv = strings.TrimLeft(kv, " ")

		tmp := strings.SplitN(kv, "=", 2)
		if len(tmp) != 2 {
			return fmt.Errorf("unable to parse key-value (%v)", kv)
		}
		k, v := tmp[0], tmp[1]

		switch k {
		case "timeout":
			iv, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return err
			}
			uiv := uint(iv)
			h.Timeout = &uiv

		default:
			// ignore non-standard keys
		}
	}

	return nil
}

// Write encodes a Session header.
func (h Session) Write() base.HeaderValue {
	ret := h.Session

	if h.Timeout != nil {
		ret += ";timeout=" + strconv.FormatUint(uint64(*h.Timeout), 10)
	}

	return base.HeaderValue{ret}
}
