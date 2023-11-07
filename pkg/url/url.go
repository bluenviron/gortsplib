// Package url is deprecated.
package url

import (
	"github.com/bluenviron/gortsplib/v4/pkg/base"
)

// URL is a RTSP URL.
// This is basically an HTTP URL with some additional functions to handle
// control attributes.
//
// Deprecated: replaced by base.URL
type URL = base.URL

// Parse parses a RTSP URL.
//
// Deprecated: replaced by base.ParseURL
func Parse(s string) (*URL, error) {
	return base.ParseURL(s)
}
