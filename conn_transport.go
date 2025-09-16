package gortsplib

// Tunnel is a tunnel method.
type Tunnel int

// tunnel methods.
const (
	TunnelNone Tunnel = iota
	TunnelHTTP
)

// ConnTransport contains details about the transport of a connection.
type ConnTransport struct {
	Tunnel Tunnel
}
