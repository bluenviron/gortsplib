package base

import (
	"fmt"
	"net/url"
)

// supported schemas
const (
	schemeRTSP  = "rtsp"
	schemeRTSPS = "rtsps"
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

	if u.Scheme != schemeRTSP && u.Scheme != schemeRTSPS {
		return nil, fmt.Errorf("unsupported scheme '%s'", u.Scheme)
	}

	if u.Opaque != "" {
		return nil, fmt.Errorf("URLs with opaque data are not supported")
	}

	if u.Fragment != "" {
		return nil, fmt.Errorf("URLs with fragments are not supported")
	}

	return (*URL)(u), nil
}

// String implements fmt.Stringer.
func (u *URL) String() string {
	return (*url.URL)(u).String()
}

// Clone clones a URL.
func (u *URL) Clone() *URL {
	return (*URL)(&url.URL{
		Scheme:     u.Scheme,
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
		Host:       u.Host,
		Path:       u.Path,
		RawPath:    u.RawPath,
		ForceQuery: u.ForceQuery,
		RawQuery:   u.RawQuery,
	})
}

// RequestURI returns the path and query of the URL (abs_path).
func (u *URL) RequestURI() string {
	return (*url.URL)(u).RequestURI()
}

// Hostname returns u.Host, stripping any valid port number if present.
//
// If the result is enclosed in square brackets, as literal IPv6 addresses are,
// the square brackets are removed from the result.
func (u *URL) Hostname() string {
	return (*url.URL)(u).Hostname()
}

// Port returns the port part of u.Host, without the leading colon.
//
// If u.Host doesn't contain a valid numeric port, Port returns an empty string.
func (u *URL) Port() string {
	return (*url.URL)(u).Port()
}
