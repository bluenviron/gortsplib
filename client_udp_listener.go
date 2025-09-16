package gortsplib

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"net"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bluenviron/gortsplib/v5/pkg/multicast"
	"github.com/bluenviron/gortsplib/v5/pkg/readbuffer"
)

func ptrOf[T any](v T) *T {
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
	SetReadBuffer(bytes int) error
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
		err := u.pc.SetReadBuffer(u.c.UDPReadBufferSize)
		if err != nil {
			u.pc.Close()
			return err
		}

		v, err := readbuffer.ReadBuffer(u.pc)
		if err != nil {
			u.pc.Close()
			return err
		}

		if v != u.c.UDPReadBufferSize {
			u.pc.Close()
			return fmt.Errorf("unable to set read buffer size to %v, check that the operating system allows that",
				u.c.UDPReadBufferSize)
		}
	}

	u.lastPacketTime = ptrOf(int64(0))
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
	if u.running {
		u.pc.SetReadDeadline(time.Now())
		<-u.done
		u.running = false
	}
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
