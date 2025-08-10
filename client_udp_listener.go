package gortsplib

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bluenviron/gortsplib/v4/pkg/multicast"
)

func int64Ptr(v int64) *int64 {
	return &v
}

func randInRange(maxVal int) (int, error) {
	b := big.NewInt(int64(maxVal + 1))
	n, err := rand.Int(rand.Reader, b)
	if err != nil {
		return 0, err
	}
	return int(n.Int64()), nil
}

type packetConn interface {
	net.PacketConn
	SyscallConn() (syscall.RawConn, error)
}

func setAndVerifyReadBufferSize(pc packetConn, v int) error {
	rawConn, err := pc.SyscallConn()
	if err != nil {
		panic(err)
	}

	var err2 error

	err = rawConn.Control(func(fd uintptr) {
		err2 = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, v)
		if err2 != nil {
			return
		}

		var v2 int
		v2, err2 = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
		if err2 != nil {
			return
		}

		if v2 != (v * 2) {
			err2 = fmt.Errorf("unable to set read buffer size to %v - check that net.core.rmem_max is greater than %v", v, v)
			return
		}
	})
	if err != nil {
		return err
	}

	if err2 != nil {
		return err2
	}

	return nil
}

type clientUDPListener struct {
	c                  *Client
	multicast          bool
	multicastInterface *net.Interface
	address            string

	pc        packetConn
	readFunc  readFunc
	readIP    net.IP
	readPort  int
	writeAddr *net.UDPAddr

	running        bool
	lastPacketTime *int64

	done chan struct{}
}

func (u *clientUDPListener) initialize() error {
	if u.multicast {
		var err error
		u.pc, err = multicast.NewSingleConn(u.multicastInterface, u.address, u.c.ListenPacket)
		if err != nil {
			return err
		}
	} else {
		tmp, err := u.c.ListenPacket(restrictNetwork("udp", u.address))
		if err != nil {
			return err
		}
		u.pc = tmp.(*net.UDPConn)
	}

	if u.c.UDPReadBufferSize != 0 {
		err := setAndVerifyReadBufferSize(u.pc, u.c.UDPReadBufferSize)
		if err != nil {
			u.pc.Close()
			return err
		}
	}

	u.lastPacketTime = int64Ptr(0)
	return nil
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
	u.running = false
}

func (u *clientUDPListener) run() {
	defer close(u.done)

	var buf []byte

	createNewBuffer := func() {
		buf = make([]byte, udpMaxPayloadSize+1)
	}

	createNewBuffer()

	for {
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

		if u.readFunc(buf[:n]) {
			createNewBuffer()
		}
	}
}

func (u *clientUDPListener) write(payload []byte) error {
	// no mutex is needed here since Write() has an internal lock.
	// https://github.com/golang/go/issues/27203#issuecomment-534386117
	u.pc.SetWriteDeadline(time.Now().Add(u.c.WriteTimeout))
	_, err := u.pc.WriteTo(payload, u.writeAddr)
	return err
}
