package base

import (
	"fmt"
	"net/url"
	"strconv"
)

func stringsReverseIndex(s, substr string) int {
	for i := len(s) - 1 - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// PathSplitControlAttribute splits a path and query from a control attribute.
func PathSplitControlAttribute(pathAndQuery string) (int, string, bool) {
	i := stringsReverseIndex(pathAndQuery, "/trackID=")

	// URL doesn't contain trackID - we assume it's track 0
	if i < 0 {
		return 0, pathAndQuery, true
	}

	tmp, err := strconv.ParseInt(pathAndQuery[i+len("/trackID="):], 10, 64)
	if err != nil || tmp < 0 {
		return 0, "", false
	}
	trackID := int(tmp)

	return trackID, pathAndQuery[:i], true
}

// PathSplitQuery splits a path from a query.
func PathSplitQuery(pathAndQuery string) (string, string) {
	i := stringsReverseIndex(pathAndQuery, "?")
	if i >= 0 {
		return pathAndQuery[:i], pathAndQuery[i:]
	}

	return pathAndQuery, ""
}

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
	pathAndQuery, ok := u.RTSPPathAndQuery()
	if !ok {
		return "", false
	}

	path, _ := PathSplitQuery(pathAndQuery)
	return path, true
}

// RTSPPathAndQuery returns the path and query of a RTSP URL.
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
