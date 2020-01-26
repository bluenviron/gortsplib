package gortsplib

import (
	"fmt"
	"regexp"
	"strings"
)

// HeaderAuth is an Authenticate or a WWWW-Authenticate header.
type HeaderAuth struct {
	Prefix string
	Values map[string]string
}

// ReadHeaderAuth parses an Authenticate or a WWW-Authenticate header.
func ReadHeaderAuth(in string) (*HeaderAuth, error) {
	a := &HeaderAuth{
		Values: make(map[string]string),
	}

	i := strings.IndexByte(in, ' ')
	if i < 0 {
		return nil, fmt.Errorf("parse error")
	}
	a.Prefix, in = in[:i], in[i+1:]

	r := regexp.MustCompile("^([a-z]+)=\"(.+?)\"(, )?")

	for len(in) > 0 {
		m := r.FindStringSubmatch(in)
		if m == nil {
			return nil, fmt.Errorf("parse error")
		}
		in = in[len(m[0]):]

		a.Values[m[1]] = m[2]
	}

	return a, nil
}
