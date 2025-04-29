package gortsplib

// Transport is a RTSP transport protocol.
type Transport int

// transport protocols.
const (
	TransportUDP Transport = iota
	TransportUDPMulticast
	TransportTCP
	TransportHTTP
)

var transportLabels = map[Transport]string{
	TransportUDP:          "UDP",
	TransportUDPMulticast: "UDP-multicast",
	TransportTCP:          "TCP",
	TransportHTTP:         "HTTP",
}

// String implements fmt.Stringer.
func (t Transport) String() string {
	if l, ok := transportLabels[t]; ok {
		return l
	}
	return "unknown"
}
