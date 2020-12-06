package gortsplib

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
	"github.com/aler9/gortsplib/pkg/multibuffer"
	"github.com/aler9/gortsplib/pkg/rtcpreceiver"
	"github.com/aler9/gortsplib/pkg/rtcpsender"
)

// DefaultClientConf is the default ClientConf.
var DefaultClientConf = ClientConf{}

// Dial connects to a server.
func Dial(host string) (*ClientConn, error) {
	return DefaultClientConf.Dial(host)
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

	// timeout of read operations.
	// It defaults to 10 seconds.
	ReadTimeout time.Duration

	// timeout of write operations.
	// It defaults to 10 seconds.
	WriteTimeout time.Duration

	// disable being redirected to other servers, that can happen during Describe().
	// It defaults to false.
	RedirectDisable bool

	// read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
	// that is reading frames.
	// It defaults to 1.
	ReadBufferCount int

	// function used to initialize the TCP client.
	// It defaults to net.DialTimeout.
	DialTimeout func(network, address string, timeout time.Duration) (net.Conn, error)

	// function used to initialize UDP listeners.
	// It defaults to net.ListenPacket.
	ListenPacket func(network, address string) (net.PacketConn, error)
}

// Dial connects to a server.
func (d ClientConf) Dial(host string) (*ClientConn, error) {
	if d.ReadTimeout == 0 {
		d.ReadTimeout = 10 * time.Second
	}
	if d.WriteTimeout == 0 {
		d.WriteTimeout = 10 * time.Second
	}
	if d.ReadBufferCount == 0 {
		d.ReadBufferCount = 1
	}
	if d.DialTimeout == nil {
		d.DialTimeout = net.DialTimeout
	}
	if d.ListenPacket == nil {
		d.ListenPacket = net.ListenPacket
	}

	if !strings.Contains(host, ":") {
		host += ":554"
	}

	nconn, err := d.DialTimeout("tcp", host, d.ReadTimeout)
	if err != nil {
		return nil, err
	}

	return &ClientConn{
		d:                 d,
		nconn:             nconn,
		br:                bufio.NewReaderSize(nconn, clientReadBufferSize),
		bw:                bufio.NewWriterSize(nconn, clientWriteBufferSize),
		udpRtpListeners:   make(map[int]*clientConnUDPListener),
		udpRtcpListeners:  make(map[int]*clientConnUDPListener),
		rtcpReceivers:     make(map[int]*rtcpreceiver.RtcpReceiver),
		udpLastFrameTimes: make(map[int]*int64),
		tcpFrameBuffer:    multibuffer.New(d.ReadBufferCount, clientTCPFrameReadBufferSize),
		rtcpSenders:       make(map[int]*rtcpsender.RtcpSender),
		publishError:      fmt.Errorf("not running"),
	}, nil
}

// DialRead connects to the address and starts reading all tracks.
func (d ClientConf) DialRead(address string) (*ClientConn, error) {
	u, err := base.ParseURL(address)
	if err != nil {
		return nil, err
	}

	conn, err := d.Dial(u.Host)
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
func (d ClientConf) DialPublish(address string, tracks Tracks) (*ClientConn, error) {
	u, err := base.ParseURL(address)
	if err != nil {
		return nil, err
	}

	conn, err := d.Dial(u.Host)
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
