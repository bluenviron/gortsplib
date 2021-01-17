package base

import (
	"fmt"
	"net/url"
)

// URL is a RTSP URL.
// This is basically an HTTP URL with some additional functions to handle
// control attributes.
type URL url.URL

// ParseURL parses a RTSP URL.
func ParseURL(s string) (*URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "rtsp" && u.Scheme != "rtsps" {
		return nil, fmt.Errorf("unsupported scheme '%s'", u.Scheme)
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

// RTSPPath returns the path of a RTSP URL.
func (u *URL) RTSPPath() (string, bool) {
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

// RTSPPathAndQuery returns the path and the query of a RTSP URL.
func (u *URL) RTSPPathAndQuery() (string, bool) {
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
		return "", false
	}
	pathAndQuery = pathAndQuery[1:]

	return pathAndQuery, true
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
