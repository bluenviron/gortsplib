package gortsplib

import "github.com/bluenviron/gortsplib/v5/pkg/headers"

// Protocol is a RTSP transport protocol.
type Protocol int

// transport protocols.
const (
	ProtocolUDP Protocol = iota
	ProtocolUDPMulticast
	ProtocolTCP
)

var transportLabels = map[Protocol]string{
	ProtocolUDP:          "UDP",
	ProtocolUDPMulticast: "UDP-multicast",
	ProtocolTCP:          "TCP",
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
