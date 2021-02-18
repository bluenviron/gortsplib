package gortsplib

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

// DefaultClientConf is the default ClientConf.
var DefaultClientConf = ClientConf{}

// Dial connects to a server.
func Dial(scheme string, host string) (*ClientConn, error) {
	return DefaultClientConf.Dial(scheme, host)
}

// DialRead connects to a server and starts reading all tracks.
func DialRead(address string) (*ClientConn, error) {
	return DefaultClientConf.DialRead(address)
}

// DialPublish connects to a server and starts publishing the tracks.
func DialPublish(address string, tracks Tracks) (*ClientConn, error) {
	return DefaultClientConf.DialPublish(address, tracks)
}

// ClientConf allows to initialize a ClientConn.
// All fields are optional.
type ClientConf struct {
	// the stream protocol (UDP or TCP).
	// If nil, it is chosen automatically (first UDP, then, if it fails, TCP).
	// It defaults to nil.
	StreamProtocol *StreamProtocol

	// a TLS configuration to connect to TLS (RTSPS) servers.
	// It defaults to &tls.Config{InsecureSkipVerify:true}
	TLSConfig *tls.Config

	// disable being redirected to other servers, that can happen during Describe().
	// It defaults to false.
	RedirectDisable bool

	// enable communication with servers which don't provide server ports.
	// this can be a security issue.
	// It defaults to false.
	AnyPortEnable bool

	// timeout of read operations.
	// It defaults to 10 seconds.
	ReadTimeout time.Duration

	// timeout of write operations.
	// It defaults to 10 seconds.
	WriteTimeout time.Duration

	// read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
	// that is reading frames.
	// It defaults to 1.
	ReadBufferCount int

	// read buffer size.
	// This must be touched only when the server reports problems about buffer sizes.
	// It defaults to 2048.
	ReadBufferSize int

	// callback called before every request.
	OnRequest func(req *base.Request)

	// callback called after very response.
	OnResponse func(res *base.Response)

	// function used to initialize the TCP client.
	// It defaults to net.DialTimeout.
	DialTimeout func(network, address string, timeout time.Duration) (net.Conn, error)

	// function used to initialize UDP listeners.
	// It defaults to net.ListenPacket.
	ListenPacket func(network, address string) (net.PacketConn, error)
}

// Dial connects to a server.
func (c ClientConf) Dial(scheme string, host string) (*ClientConn, error) {
	return newClientConn(c, scheme, host)
}

// DialRead connects to the address and starts reading all tracks.
func (c ClientConf) DialRead(address string) (*ClientConn, error) {
	u, err := base.ParseURL(address)
	if err != nil {
		return nil, err
	}

	conn, err := c.Dial(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}

	_, err = conn.Options(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	tracks, _, err := conn.Describe(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	for _, track := range tracks {
		_, err := conn.Setup(headers.TransportModePlay, track, 0, 0)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	_, err = conn.Play()
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// DialPublish connects to the address and starts publishing the tracks.
func (c ClientConf) DialPublish(address string, tracks Tracks) (*ClientConn, error) {
	u, err := base.ParseURL(address)
	if err != nil {
		return nil, err
	}

	conn, err := c.Dial(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}

	_, err = conn.Options(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	_, err = conn.Announce(u, tracks)
	if err != nil {
		conn.Close()
		return nil, err
	}

	for _, track := range tracks {
		_, err := conn.Setup(headers.TransportModeRecord, track, 0, 0)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	_, err = conn.Record()
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}
