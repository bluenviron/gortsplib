package base

import (
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

// URLGetBasePath returns the base path of a RTSP URL.
// We assume that the URL doesn't contain a control path.
func URLGetBasePath(u *url.URL) (string, bool) {
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

// URLGetBaseControlPath returns the base path and the control path of a RTSP URL.
// We assume that the URL contains a control path.
func URLGetBaseControlPath(u *url.URL) (string, string, bool) {
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

// URLAddControlPath adds a control path to a RTSP url.
func URLAddControlPath(u *url.URL, controlPath string) {
	// always insert the control path at the end of the url
	if u.RawQuery != "" {
		if !strings.HasSuffix(u.RawQuery, "/") {
			u.RawQuery += "/"
		}
		u.RawQuery += controlPath

	} else {
		if u.RawPath != "" {
			if !strings.HasSuffix(u.RawPath, "/") {
				u.RawPath += "/"
			}
			u.RawPath += controlPath
		}

		if !strings.HasSuffix(u.Path, "/") {
			u.Path += "/"
		}
		u.Path += controlPath
	}
}
