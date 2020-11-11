package gortsplib

import (
	"bufio"
	"net"
	"strings"
	"time"

	"github.com/aler9/gortsplib/base"
	"github.com/aler9/gortsplib/headers"
	"github.com/aler9/gortsplib/multibuffer"
	"github.com/aler9/gortsplib/rtcpreceiver"
)

// DefaultDialer is the default dialer, used by Dial, DialRead and DialPublish.
var DefaultDialer = Dialer{}

// Dial connects to a server.
func Dial(host string) (*ConnClient, error) {
	return DefaultDialer.Dial(host)
}

// DialRead connects to a server and starts reading all tracks.
func DialRead(address string, proto StreamProtocol) (*ConnClient, error) {
	return DefaultDialer.DialRead(address, proto)
}

// DialPublish connects to a server and starts publishing the tracks.
func DialPublish(address string, proto StreamProtocol, tracks Tracks) (*ConnClient, error) {
	return DefaultDialer.DialPublish(address, proto, tracks)
}

// Dialer allows to initialize a ConnClient.
type Dialer struct {
	// (optional) timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration

	// (optional) timeout of write operations.
	// It defaults to 5 seconds
	WriteTimeout time.Duration

	// (optional) read buffer count.
	// If greater than 1, allows to pass buffers to routines different than the one
	// that is reading frames.
	// It defaults to 1
	ReadBufferCount int

	// (optional) function used to initialize the TCP client.
	// It defaults to net.DialTimeout
	DialTimeout func(network, address string, timeout time.Duration) (net.Conn, error)

	// (optional) function used to initialize UDP listeners.
	// It defaults to net.ListenPacket
	ListenPacket func(network, address string) (net.PacketConn, error)
}

// Dial connects to a server.
func (d Dialer) Dial(host string) (*ConnClient, error) {
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

	return &ConnClient{
		d:                 d,
		nconn:             nconn,
		br:                bufio.NewReaderSize(nconn, clientReadBufferSize),
		bw:                bufio.NewWriterSize(nconn, clientWriteBufferSize),
		rtcpReceivers:     make(map[int]*rtcpreceiver.RtcpReceiver),
		udpLastFrameTimes: make(map[int]*int64),
		udpRtpListeners:   make(map[int]*connClientUDPListener),
		udpRtcpListeners:  make(map[int]*connClientUDPListener),
		response:          &base.Response{},
		frame:             &base.InterleavedFrame{},
		tcpFrameBuffer:    multibuffer.New(d.ReadBufferCount, clientTCPFrameReadBufferSize),
	}, nil
}

// DialRead connects to the address and starts reading all tracks.
func (d Dialer) DialRead(address string, proto StreamProtocol) (*ConnClient, error) {
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

	tracks, res, err := conn.Describe(u)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if res.StatusCode >= base.StatusMovedPermanently &&
		res.StatusCode <= base.StatusUseProxy {
		conn.Close()
		return d.DialRead(res.Header["Location"][0], proto)
	}

	for _, track := range tracks {
		_, err := conn.Setup(u, headers.TransportModePlay, proto, track, 0, 0)
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
func (d Dialer) DialPublish(address string, proto StreamProtocol, tracks Tracks) (*ConnClient, error) {
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
		_, err = conn.Setup(u, headers.TransportModeRecord, proto, track, 0, 0)
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
