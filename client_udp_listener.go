package gortsplib

import (
	"crypto/rand"
	"math/big"
	"net"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/multicast"
)

func int64Ptr(v int64) *int64 {
	return &v
}

func randInRange(max int) (int, error) {
	b := big.NewInt(int64(max + 1))
	n, err := rand.Int(rand.Reader, b)
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

type clientUDPListener struct {
	c  *Client
	pc net.PacketConn

	readFunc  readFunc
	readIP    net.IP
	readPort  int
	writeAddr *net.UDPAddr

	running        bool
	lastPacketTime *int64

	done chan struct{}
}

func newClientUDPListenerPair(c *Client) (*clientUDPListener, *clientUDPListener, error) {
	// choose two consecutive ports in range 65535-10000
	// RTP port must be even and RTCP port odd
	for {
		v, err := randInRange((65535 - 10000) / 2)
		if err != nil {
			return nil, nil, err
		}

		rtpPort := v*2 + 10000
		rtpListener, err := newClientUDPListener(
			c,
			false,
			nil,
			net.JoinHostPort("", strconv.FormatInt(int64(rtpPort), 10)),
		)
		if err != nil {
			continue
		}

		rtcpPort := rtpPort + 1
		rtcpListener, err := newClientUDPListener(
			c,
			false,
			nil,
			net.JoinHostPort("", strconv.FormatInt(int64(rtcpPort), 10)),
		)
		if err != nil {
			rtpListener.close()
			continue
		}

		return rtpListener, rtcpListener, nil
	}
}

type packetConn interface {
	net.PacketConn
	SetReadBuffer(int) error
}

func newClientUDPListener(
	c *Client,
	multicastEnable bool,
	multicastSourceIP net.IP,
	address string,
) (*clientUDPListener, error) {
	var pc packetConn
	if multicastEnable {
		intf, err := multicast.InterfaceForSource(multicastSourceIP)
		if err != nil {
			return nil, err
		}

		pc, err = multicast.NewSingleConn(intf, address, c.ListenPacket)
		if err != nil {
			return nil, err
		}
	} else {
		tmp, err := c.ListenPacket(restrictNetwork("udp", address))
		if err != nil {
			return nil, err
		}
		pc = tmp.(*net.UDPConn)
	}

	err := pc.SetReadBuffer(udpKernelReadBufferSize)
	if err != nil {
		pc.Close()
		return nil, err
	}

	return &clientUDPListener{
		c:              c,
		pc:             pc,
		lastPacketTime: int64Ptr(0),
	}, nil
}

func (u *clientUDPListener) close() {
	if u.running {
		u.stop()
	}
	u.pc.Close()
}

func (u *clientUDPListener) port() int {
	return u.pc.LocalAddr().(*net.UDPAddr).Port
}

func (u *clientUDPListener) start() {
	u.running = true
	u.pc.SetReadDeadline(time.Time{})
	u.done = make(chan struct{})
	go u.run()
}

func (u *clientUDPListener) stop() {
	u.pc.SetReadDeadline(time.Now())
	<-u.done
}

func (u *clientUDPListener) run() {
	defer close(u.done)

	for {
		buf := make([]byte, udpMaxPayloadSize+1)
		n, addr, err := u.pc.ReadFrom(buf)
		if err != nil {
			return
		}

		uaddr := addr.(*net.UDPAddr)

		if !u.readIP.Equal(uaddr.IP) {
			continue
		}

		// in case of anyPortEnable, store the port of the first packet we receive.
		// this reduces security issues
		if u.c.AnyPortEnable && u.readPort == 0 {
			u.readPort = uaddr.Port
		} else if u.readPort != uaddr.Port {
			continue
		}

		now := u.c.timeNow()
		atomic.StoreInt64(u.lastPacketTime, now.Unix())

		u.readFunc(buf[:n])
	}
}

func (u *clientUDPListener) write(payload []byte) error {
	// no mutex is needed here since Write() has an internal lock.
	// https://github.com/golang/go/issues/27203#issuecomment-534386117
	u.pc.SetWriteDeadline(time.Now().Add(u.c.WriteTimeout))
	_, err := u.pc.WriteTo(payload, u.writeAddr)
	return err
}
