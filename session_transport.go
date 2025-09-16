package gortsplib

import "github.com/bluenviron/gortsplib/v5/pkg/headers"

// Protocol is a RTSP transport protocol.
type Protocol int

// transport protocols.
const (
	TransportUDP Protocol = iota
	TransportUDPMulticast
	TransportTCP
)

var transportLabels = map[Protocol]string{
	TransportUDP:          "UDP",
	TransportUDPMulticast: "UDP-multicast",
	TransportTCP:          "TCP",
}

// String implements fmt.Stringer.
func (t Protocol) String() string {
	if l, ok := transportLabels[t]; ok {
		return l
	}
	return "unknown"
}

// SessionTransport contains details about the transport of a session.
type SessionTransport struct {
	Protocol Protocol
	Profile  headers.TransportProfile
}
