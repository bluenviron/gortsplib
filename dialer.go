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
)

// DefaultDialer is the default dialer, used by Dial, DialRead and DialPublish.
var DefaultDialer = Dialer{}

// Dial connects to a server.
func Dial(host string) (*ConnClient, error) {
	return DefaultDialer.Dial(host)
}

// DialRead connects to a server and starts reading all tracks.
func DialRead(address string) (*ConnClient, error) {
	return DefaultDialer.DialRead(address)
}

// DialPublish connects to a server and starts publishing the tracks.
func DialPublish(address string, tracks Tracks) (*ConnClient, error) {
	return DefaultDialer.DialPublish(address, tracks)
}

// Dialer allows to initialize a ConnClient.
type Dialer struct {
	// (optional) the stream protocol (UDP or TCP).
	// If nil, it is chosen automatically (first UDP, then, if it fails, TCP).
	StreamProtocol *StreamProtocol

	// (optional) timeout of read operations.
	// It defaults to 10 seconds
	ReadTimeout time.Duration

	// (optional) timeout of write operations.
	// It defaults to 10 seconds
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
		tcpFrameBuffer:    multibuffer.New(d.ReadBufferCount, clientTCPFrameReadBufferSize),
		publishError:      fmt.Errorf("not running"),
	}, nil
}

// DialRead connects to the address and starts reading all tracks.
func (d Dialer) DialRead(address string) (*ConnClient, error) {
	u, err := base.ParseURL(address)
	if err != nil {
		return nil, err
	}

	conn, err := d.Dial(u.Host)
	if err != nil {
		return nil, err
	}

	res, err := conn.Options(u)
	if err != nil {
		// since this method is not implemented by every RTSP server,
		// return only if status code is not 404
		if res == nil || res.StatusCode != base.StatusNotFound {
			conn.Close()
			return nil, err
		}
	}

	tracks, res, err := conn.Describe(u)
	if err != nil {
		// redirect
		if res != nil && res.StatusCode >= base.StatusMovedPermanently &&
			res.StatusCode <= base.StatusUseProxy &&
			len(res.Header["Location"]) == 1 {
			conn.Close()
			return d.DialRead(res.Header["Location"][0])
		}

		conn.Close()
		return nil, err
	}

	proto := func() StreamProtocol {
		if d.StreamProtocol != nil {
			return *d.StreamProtocol
		}
		return StreamProtocolUDP
	}()

	for i, track := range tracks {
		res, err := conn.Setup(u, headers.TransportModePlay, proto, track, 0, 0)
		if err != nil {
			// switch protocol automatically
			if i == 0 && d.StreamProtocol == nil && res != nil &&
				res.StatusCode == base.StatusUnsupportedTransport {
				proto = StreamProtocolTCP
				_, err := conn.Setup(u, headers.TransportModePlay, proto, track, 0, 0)
				if err != nil {
					conn.Close()
					return nil, err
				}
			} else {
				conn.Close()
				return nil, err
			}
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
func (d Dialer) DialPublish(address string, tracks Tracks) (*ConnClient, error) {
	u, err := base.ParseURL(address)
	if err != nil {
		return nil, err
	}

	conn, err := d.Dial(u.Host)
	if err != nil {
		return nil, err
	}

	res, err := conn.Options(u)
	if err != nil {
		// since this method is not implemented by every RTSP server,
		// return only if status code is not 404
		if res == nil || res.StatusCode != base.StatusNotFound {
			conn.Close()
			return nil, err
		}
	}

	_, err = conn.Announce(u, tracks)
	if err != nil {
		conn.Close()
		return nil, err
	}

	proto := func() StreamProtocol {
		if d.StreamProtocol != nil {
			return *d.StreamProtocol
		}
		return StreamProtocolUDP
	}()

	for i, track := range tracks {
		res, err := conn.Setup(u, headers.TransportModeRecord, proto, track, 0, 0)
		if err != nil {
			// switch protocol automatically
			if i == 0 && d.StreamProtocol == nil && res != nil &&
				res.StatusCode == base.StatusUnsupportedTransport {
				proto = StreamProtocolTCP
				_, err := conn.Setup(u, headers.TransportModePlay, proto, track, 0, 0)
				if err != nil {
					conn.Close()
					return nil, err
				}
			} else {
				conn.Close()
				return nil, err
			}
		}
	}

	_, err = conn.Record()
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}
