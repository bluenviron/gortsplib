package gortsplib

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// HeaderAuth is an Authenticate or a WWWW-Authenticate header.
type HeaderAuth struct {
	Prefix string
	Values map[string]string
}

// ReadHeaderAuth parses an Authenticate or a WWW-Authenticate header.
func ReadHeaderAuth(in string) (*HeaderAuth, error) {
	ha := &HeaderAuth{
		Values: make(map[string]string),
	}

	i := strings.IndexByte(in, ' ')
	if i < 0 {
		return nil, fmt.Errorf("unable to find prefix (%s)", in)
	}
	ha.Prefix, in = in[:i], in[i+1:]

	r := regexp.MustCompile("^([a-z]+)=(\"(.+?)\"|([a-zA-Z0-9]+))(, )?")

	for len(in) > 0 {
		m := r.FindStringSubmatch(in)
		if m == nil {
			return nil, fmt.Errorf("unable to parse key-value (%s)", in)
		}
		in = in[len(m[0]):]

		m[2] = strings.TrimPrefix(m[2], "\"")
		m[2] = strings.TrimSuffix(m[2], "\"")
		ha.Values[m[1]] = m[2]
	}

	return ha, nil
}

// Write encodes an Authenticate or a WWW-Authenticate header.
func (ha *HeaderAuth) Write() string {
	ret := ha.Prefix + " "

	var sortedKeys []string
	for key := range ha.Values {
		sortedKeys = append(sortedKeys, key)
	}
	sort.Strings(sortedKeys)

	var tmp []string
	for _, key := range sortedKeys {
		tmp = append(tmp, key+"=\""+ha.Values[key]+"\"")
	}
	ret += strings.Join(tmp, ", ")

	return ret
}
