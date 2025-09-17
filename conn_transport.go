package gortsplib

// Tunnel is a tunneling method.
type Tunnel int

// tunneling methods.
const (
	TunnelNone Tunnel = iota
	TunnelHTTP
	TunnelWebSocket
)

// ConnTransport contains details about the transport of a connection.
type ConnTransport struct {
	Tunnel Tunnel
}
