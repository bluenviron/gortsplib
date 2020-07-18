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

var regHeaderAuthKeyValue = regexp.MustCompile("^([a-z]+)=(\"(.*?)\"|([a-zA-Z0-9]+))(, *|$)")

// ReadHeaderAuth parses an Authenticate or a WWW-Authenticate header.
func ReadHeaderAuth(v HeaderValue) (*HeaderAuth, error) {
	if len(v) == 0 {
		return nil, fmt.Errorf("value not provided")
	}

	if len(v) > 1 {
		return nil, fmt.Errorf("value provided multiple times (%v)", v)
	}

	ha := &HeaderAuth{
		Values: make(map[string]string),
	}

	v0 := v[0]

	i := strings.IndexByte(v[0], ' ')
	if i < 0 {
		return nil, fmt.Errorf("unable to find prefix (%s)", v0)
	}
	ha.Prefix, v0 = v0[:i], v0[i+1:]

	for len(v0) > 0 {
		m := regHeaderAuthKeyValue.FindStringSubmatch(v0)
		if m == nil {
			return nil, fmt.Errorf("unable to parse key-value (%s)", v0)
		}
		v0 = v0[len(m[0]):]

		m[2] = strings.TrimPrefix(m[2], "\"")
		m[2] = strings.TrimSuffix(m[2], "\"")
		ha.Values[m[1]] = m[2]
	}

	return ha, nil
}

// Write encodes an Authenticate or a WWW-Authenticate header.
func (ha *HeaderAuth) Write() HeaderValue {
	ret := ha.Prefix + " "

	// always put realm first, otherwise VLC does not send back the response
	var sortedKeys []string
	for key := range ha.Values {
		if key != "realm" {
			sortedKeys = append(sortedKeys, key)
		}
	}
	sort.Strings(sortedKeys)
	if _, ok := ha.Values["realm"]; ok {
		sortedKeys = append([]string{"realm"}, sortedKeys...)
	}

	var tmp []string
	for _, key := range sortedKeys {
		tmp = append(tmp, key+"=\""+ha.Values[key]+"\"")
	}
	ret += strings.Join(tmp, ", ")

	return HeaderValue{ret}
}
