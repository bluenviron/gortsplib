package gortsplib

import "github.com/bluenviron/gortsplib/v5/pkg/headers"

// TransportProtocol is a RTSP transport protocol.
type TransportProtocol int

// transport protocols.
const (
	TransportUDP TransportProtocol = iota
	TransportUDPMulticast
	TransportTCP
)

var transportLabels = map[TransportProtocol]string{
	TransportUDP:          "UDP",
	TransportUDPMulticast: "UDP-multicast",
	TransportTCP:          "TCP",
}

// String implements fmt.Stringer.
func (t TransportProtocol) String() string {
	if l, ok := transportLabels[t]; ok {
		return l
	}
	return "unknown"
}

// SessionTransport contains details about the transport of a session.
type SessionTransport struct {
	Protocol TransportProtocol
	Profile  headers.TransportProfile
}
