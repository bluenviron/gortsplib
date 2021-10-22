/*
Package gortsplib is a RTSP 1.0 library for the Go programming language,
written for rtsp-simple-server.

Examples are available at https://github.com/aler9/gortsplib/tree/master/examples

*/
package gortsplib

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

// DefaultClient is the default Client.
var DefaultClient = &Client{}

// Dial connects to a server.
func Dial(scheme string, host string) (*ClientConn, error) {
	return DefaultClient.Dial(scheme, host)
}

// DialRead connects to a server and starts reading all tracks.
func DialRead(address string) (*ClientConn, error) {
	return DefaultClient.DialRead(address)
}

// DialPublish connects to a server and starts publishing the tracks.
func DialPublish(address string, tracks Tracks) (*ClientConn, error) {
	return DefaultClient.DialPublish(address, tracks)
}

// ClientTransport is a stream transport used by the client.
type ClientTransport int

// standard client transports.
const (
	ClientTransportUDP ClientTransport = iota
	ClientTransportMulticast
	ClientTransportTCP
)

// Client is a RTSP client.
type Client struct {
	//
	// connection
	//
	// timeout of read operations.
	// It defaults to 10 seconds.
	ReadTimeout time.Duration
	// timeout of write operations.
	// It defaults to 10 seconds.
	WriteTimeout time.Duration
	// a TLS configuration to connect to TLS (RTSPS) servers.
	// It defaults to &tls.Config{InsecureSkipVerify:true}
	TLSConfig *tls.Config

	//
	// initialization
	//
	// disable being redirected to other servers, that can happen during Describe().
	// It defaults to false.
	RedirectDisable bool
	// enable communication with servers which don't provide server ports.
	// this can be a security issue.
	// It defaults to false.
	AnyPortEnable bool

	//
	// reading / writing
	//
	// the stream transport (UDP, Multicast or TCP).
	// If nil, it is chosen automatically (first UDP, then, if it fails, TCP).
	// It defaults to nil.
	Transport *ClientTransport
	// If the client is reading with UDP, it must receive
	// at least a packet within this timeout.
	// It defaults to 3 seconds.
	InitialUDPReadTimeout time.Duration
	// read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
	// that is reading frames.
	// It defaults to 1.
	ReadBufferCount int
	// read buffer size.
	// This must be touched only when the server reports problems about buffer sizes.
	// It defaults to 2048.
	ReadBufferSize int

	//
	// callbacks
	//
	// callback called before every request.
	OnRequest func(req *base.Request)
	// callback called after every response.
	OnResponse func(res *base.Response)

	//
	// system functions
	//
	// function used to initialize the TCP client.
	// It defaults to (&net.Dialer{}).DialContext.
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)
	// function used to initialize UDP listeners.
	// It defaults to net.ListenPacket.
	ListenPacket func(network, address string) (net.PacketConn, error)

	//
	// private
	//

	senderReportPeriod   time.Duration
	receiverReportPeriod time.Duration
}

// Dial connects to a server.
func (c *Client) Dial(scheme string, host string) (*ClientConn, error) {
	return newClientConn(c, scheme, host)
}

// DialRead connects to the address and starts reading all tracks.
func (c *Client) DialRead(address string) (*ClientConn, error) {
	return c.DialReadContext(context.Background(), address)
}

// DialReadContext connects to the address with the given context and starts reading all tracks.
func (c *Client) DialReadContext(ctx context.Context, address string) (*ClientConn, error) {
	u, err := base.ParseURL(address)
	if err != nil {
		return nil, err
	}

	conn, err := c.Dial(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}

	ctxHandlerDone := make(chan struct{})
	defer func() { <-ctxHandlerDone }()

	ctxHandlerTerminate := make(chan struct{})
	defer close(ctxHandlerTerminate)

	go func() {
		defer close(ctxHandlerDone)
		select {
		case <-ctx.Done():
			conn.Close()
		case <-ctxHandlerTerminate:
		}
	}()

	_, err = conn.Options(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	tracks, baseURL, _, err := conn.Describe(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	for _, track := range tracks {
		_, err := conn.Setup(headers.TransportModePlay, baseURL, track, 0, 0)
		if err != nil {
			conn.Close()
			return nil, err
		}
	}

	_, err = conn.Play(nil)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// DialPublish connects to the address and starts publishing the tracks.
func (c *Client) DialPublish(address string, tracks Tracks) (*ClientConn, error) {
	return c.DialPublishContext(context.Background(), address, tracks)
}

// DialPublishContext connects to the address with the given context and starts publishing the tracks.
func (c *Client) DialPublishContext(ctx context.Context, address string, tracks Tracks) (*ClientConn, error) {
	u, err := base.ParseURL(address)
	if err != nil {
		return nil, err
	}

	conn, err := c.Dial(u.Scheme, u.Host)
	if err != nil {
		return nil, err
	}

	ctxHandlerDone := make(chan struct{})
	defer func() { <-ctxHandlerDone }()

	ctxHandlerTerminate := make(chan struct{})
	defer close(ctxHandlerTerminate)

	go func() {
		defer close(ctxHandlerDone)
		select {
		case <-ctx.Done():
			conn.Close()
		case <-ctxHandlerTerminate:
		}
	}()

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
		_, err := conn.Setup(headers.TransportModeRecord, u, track, 0, 0)
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
