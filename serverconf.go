package gortsplib

import (
	"crypto/tls"
	"net"
	"time"
)

// DefaultServerConf is the default ServerConf.
var DefaultServerConf = ServerConf{}

// Serve starts a server on the given address.
func Serve(address string) (*Server, error) {
	return DefaultServerConf.Serve(address)
}

// ServerConf allows to configure a Server.
// All fields are optional.
type ServerConf struct {
	// A TLS configuration to accept TLS (RTSPS) connections.
	TLSConfig *tls.Config

	// A ServerUDPListener to send and receive UDP/RTP packets.
	// If UDPRTPListener and UDPRTCPListener are not null, the server can accept and send UDP streams.
	UDPRTPListener *ServerUDPListener

	// A ServerUDPListener to send and receive UDP/RTCP packets.
	// If UDPRTPListener and UDPRTCPListener are not null, the server can accept and send UDP streams.
	UDPRTCPListener *ServerUDPListener

	// Timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration

	// Timeout of write operations.
	// It defaults to 10 seconds
	WriteTimeout time.Duration

	// Read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
	// that is reading frames.
	// It also allows to buffer routed frames and mitigate network fluctuations
	// that are particularly high when using UDP.
	// It defaults to 512
	ReadBufferCount uint64

	// Function used to initialize the TCP listener.
	// It defaults to net.Listen
	Listen func(network string, address string) (net.Listener, error)
}

// Serve starts a server on the given address.
func (c ServerConf) Serve(address string) (*Server, error) {
	return newServer(c, address)
}
