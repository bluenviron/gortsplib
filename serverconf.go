package gortsplib

import (
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
	// timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration

	// timeout of write operations.
	// It defaults to 10 seconds
	WriteTimeout time.Duration

	// read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
	// that is reading frames.
	// It defaults to 1
	ReadBufferCount int

	// function used to initialize the TCP listener.
	// It defaults to net.ListenTCP.
	ListenTCP func(network string, address *net.TCPAddr) (*net.TCPListener, error)
}

// Serve starts a server on the given address.
func (c ServerConf) Serve(address string) (*Server, error) {
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 10 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 10 * time.Second
	}
	if c.ReadBufferCount == 0 {
		c.ReadBufferCount = 1
	}
	if c.ListenTCP == nil {
		c.ListenTCP = net.ListenTCP
	}

	addr, err := net.ResolveTCPAddr("tcp", address)
	if err != nil {
		return nil, err
	}

	listener, err := c.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}

	s := &Server{
		conf:     c,
		listener: listener,
	}

	return s, nil
}
