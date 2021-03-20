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

	for _, part := range parts[1:] {
		// remove leading spaces
		part = strings.TrimLeft(part, " ")

		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			return fmt.Errorf("invalid value")
		}

		key, strValue := kv[0], kv[1]
		if key != "timeout" {
			return fmt.Errorf("invalid key '%s'", key)
		}

		iv, err := strconv.ParseUint(strValue, 10, 64)
		if err != nil {
			return err
		}
		uiv := uint(iv)

		h.Timeout = &uiv
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
