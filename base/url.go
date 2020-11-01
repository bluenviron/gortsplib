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
type URL struct {
	inner *url.URL
}

// ParseURL parses a RTSP URL.
func ParseURL(s string) (*URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "rtsp" {
		return nil, fmt.Errorf("wrong scheme")
	}

	return &URL{u}, nil
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
	return u.inner.String()
}

// Clone clones a URL.
func (u *URL) Clone() *URL {
	return &URL{&url.URL{
		Scheme:     u.inner.Scheme,
		Opaque:     u.inner.Opaque,
		User:       u.inner.User,
		Host:       u.inner.Host,
		Path:       u.inner.Path,
		RawPath:    u.inner.RawPath,
		ForceQuery: u.inner.ForceQuery,
		RawQuery:   u.inner.RawQuery,
	}}
}

// CloneWithoutCredentials clones a URL without its credentials.
func (u *URL) CloneWithoutCredentials() *URL {
	return &URL{&url.URL{
		Scheme:     u.inner.Scheme,
		Opaque:     u.inner.Opaque,
		Host:       u.inner.Host,
		Path:       u.inner.Path,
		RawPath:    u.inner.RawPath,
		ForceQuery: u.inner.ForceQuery,
		RawQuery:   u.inner.RawQuery,
	}}
}

// Host returns the host of a RTSP URL.
func (u *URL) Host() string {
	return u.inner.Host
}

// User returns the credentials of a RTSP URL.
func (u *URL) User() *url.Userinfo {
	return u.inner.User
}

// BasePath returns the base path of a RTSP URL.
// We assume that the URL doesn't contain a control path.
func (u *URL) BasePath() (string, bool) {
	var path string
	if u.inner.RawPath != "" {
		path = u.inner.RawPath
	} else {
		path = u.inner.Path
	}

	// remove leading slash
	if len(path) == 0 || path[0] != '/' {
		return "", false
	}
	path = path[1:]

	return path, true
}

// BaseControlPath returns the base path and the control path of a RTSP URL.
// We assume that the URL contains a control path.
func (u *URL) BaseControlPath() (string, string, bool) {
	var pathAndQuery string
	if u.inner.RawPath != "" {
		pathAndQuery = u.inner.RawPath
	} else {
		pathAndQuery = u.inner.Path
	}
	if u.inner.RawQuery != "" {
		pathAndQuery += "?" + u.inner.RawQuery
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

// SetHost sets the host of a RTSP URL.
func (u *URL) SetHost(host string) {
	u.inner.Host = host
}

// SetUser sets the credentials of a RTSP URL.
func (u *URL) SetUser(user *url.Userinfo) {
	u.inner.User = user
}

// AddControlPath adds a control path to a RTSP url.
func (u *URL) AddControlPath(controlPath string) {
	// always insert the control path at the end of the url
	if u.inner.RawQuery != "" {
		if !strings.HasSuffix(u.inner.RawQuery, "/") {
			u.inner.RawQuery += "/"
		}
		u.inner.RawQuery += controlPath

	} else {
		if u.inner.RawPath != "" {
			if !strings.HasSuffix(u.inner.RawPath, "/") {
				u.inner.RawPath += "/"
			}
			u.inner.RawPath += controlPath
		}

		if !strings.HasSuffix(u.inner.Path, "/") {
			u.inner.Path += "/"
		}
		u.inner.Path += controlPath
	}
}

// RemoveControlPath removes a control path from an URL.
func (u *URL) RemoveControlPath() {
	_, controlPath, ok := u.BaseControlPath()
	if !ok {
		return
	}

	if strings.HasSuffix(u.inner.RawQuery, controlPath) {
		u.inner.RawQuery = u.inner.RawQuery[:len(u.inner.RawQuery)-len(controlPath)]

	} else if strings.HasSuffix(u.inner.RawPath, controlPath) {
		u.inner.RawPath = u.inner.RawPath[:len(u.inner.RawPath)-len(controlPath)]

	} else if strings.HasSuffix(u.inner.Path, controlPath) {
		u.inner.Path = u.inner.Path[:len(u.inner.Path)-len(controlPath)]
	}
}
