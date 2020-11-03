package base

import (
	"fmt"
	"net/url"
	"strings"
)

func stringsReverseIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// URL is a RTSP URL.
// This is basically an HTTP url with some additional functions to handle
// control attributes.
type URL url.URL

// ParseURL parses a RTSP URL.
func ParseURL(s string) (*URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "rtsp" {
		return nil, fmt.Errorf("wrong scheme")
	}

	return (*URL)(u), nil
}

// MustParseURL is like ParseURL but panics in case of errors.
func MustParseURL(s string) *URL {
	u, err := ParseURL(s)
	if err != nil {
		panic(err)
	}
	return u
}

// String implements fmt.Stringer.
func (u *URL) String() string {
	return (*url.URL)(u).String()
}

// Clone clones a URL.
func (u *URL) Clone() *URL {
	return (*URL)(&url.URL{
		Scheme:     u.Scheme,
		Opaque:     u.Opaque,
		User:       u.User,
		Host:       u.Host,
		Path:       u.Path,
		RawPath:    u.RawPath,
		ForceQuery: u.ForceQuery,
		RawQuery:   u.RawQuery,
	})
}

// CloneWithoutCredentials clones a URL without its credentials.
func (u *URL) CloneWithoutCredentials() *URL {
	return (*URL)(&url.URL{
		Scheme:     u.Scheme,
		Opaque:     u.Opaque,
		Host:       u.Host,
		Path:       u.Path,
		RawPath:    u.RawPath,
		ForceQuery: u.ForceQuery,
		RawQuery:   u.RawQuery,
	})
}

// BasePath returns the base path of a RTSP URL.
// We assume that the URL doesn't contain a control attribute.
func (u *URL) BasePath() (string, bool) {
	var path string
	if u.RawPath != "" {
		path = u.RawPath
	} else {
		path = u.Path
	}

	// remove leading slash
	if len(path) == 0 || path[0] != '/' {
		return "", false
	}
	path = path[1:]

	return path, true
}

// BasePathControlAttr returns the base path and the control attribute of a RTSP URL.
// We assume that the URL contains a control attribute.
// We assume that the base path and control attribute are divided with a slash.
func (u *URL) BasePathControlAttr() (string, string, bool) {
	var pathAndQuery string
	if u.RawPath != "" {
		pathAndQuery = u.RawPath
	} else {
		pathAndQuery = u.Path
	}
	if u.RawQuery != "" {
		pathAndQuery += "?" + u.RawQuery
	}

	// remove leading slash
	if len(pathAndQuery) == 0 || pathAndQuery[0] != '/' {
		return "", "", false
	}
	pathAndQuery = pathAndQuery[1:]

	pos := stringsReverseIndexByte(pathAndQuery, '/')
	if pos < 0 {
		return "", "", false
	}

	basePath := pathAndQuery[:pos]

	// remove query from basePath
	i := strings.IndexByte(basePath, '?')
	if i >= 0 {
		basePath = basePath[:i]
	}

	if len(basePath) == 0 {
		return "", "", false
	}

	controlPath := pathAndQuery[pos+1:]
	if len(controlPath) == 0 {
		return "", "", false
	}

	return basePath, controlPath, true
}

// AddControlAttribute adds a control attribute to a RTSP url.
func (u *URL) AddControlAttribute(controlPath string) {
	if controlPath[0] != '?' {
		controlPath = "/" + controlPath
	}

	// insert the control attribute at the end of the url
	// if there's a query, insert it after the query
	// otherwise insert it after the path
	nu, _ := ParseURL(u.String() + controlPath)
	*u = *nu
}

// RemoveControlAttribute removes a control attribute from an URL.
func (u *URL) RemoveControlAttribute() {
	_, controlPath, ok := u.BasePathControlAttr()
	if !ok {
		return
	}

	urStr := u.String()
	nu, _ := ParseURL(urStr[:len(urStr)-len(controlPath)])
	*u = *nu
}
