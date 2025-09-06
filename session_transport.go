package gortsplib

import "github.com/bluenviron/gortsplib/v4/pkg/headers"

// SessionTransport contains details about the transport of a session.
type SessionTransport struct {
	Protocol TransportProtocol
	Profile  headers.TransportProfile
}
