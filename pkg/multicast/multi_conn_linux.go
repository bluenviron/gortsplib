package multicast

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"time"
)

// multiConn is a multicast connection
// that works in parallel on all interfaces.
type multiConn struct {
	addr       *net.UDPAddr
	readFile   *os.File
	readConn   net.PacketConn
	writeFiles []*os.File
	writeConns []net.PacketConn
}

// NewMultiConn allocates a multiConn.
func NewMultiConn(
	address string,
	readOnly bool,
	_ func(network, address string) (net.PacketConn, error),
) (Conn, error) {
	addr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, err
	}

	var gi genericIP
	if addr.IP.To4() == nil {
		gi.Family = syscall.AF_INET6
		gi.IPProto = syscall.IPPROTO_IPV6
		gi.MulticastHops = syscall.IPV6_MULTICAST_HOPS
	} else {
		gi.Family = syscall.AF_INET
		gi.IPProto = syscall.IPPROTO_IP
		gi.MulticastHops = syscall.IP_MULTICAST_TTL
	}

	readSock, err := syscall.Socket(gi.Family, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		return nil, err
	}

	err = syscall.SetsockoptInt(readSock, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
	if err != nil {
		syscall.Close(readSock) //nolint:errcheck
		return nil, err
	}

	var lsa6 syscall.SockaddrInet6
	var lsa4 syscall.SockaddrInet4

	if gi.Family == syscall.AF_INET6 {
		lsa6.Port = addr.Port
		copy(lsa6.Addr[:], addr.IP.To16())
		err = syscall.Bind(readSock, &lsa6)
	} else {
		lsa4.Port = addr.Port
		copy(lsa4.Addr[:], addr.IP.To4())
		err = syscall.Bind(readSock, &lsa4)
	}
	if err != nil {
		syscall.Close(readSock) //nolint:errcheck
		return nil, err
	}

	intfs, err := net.Interfaces()
	if err != nil {
		syscall.Close(readSock) //nolint:errcheck
		return nil, err
	}

	var enabledInterfaces []*net.Interface //nolint:prealloc

	for _, intf := range intfs {
		if (intf.Flags & net.FlagMulticast) == 0 {
			continue
		}

		if gi.Family == syscall.AF_INET6 {
			var mreq syscall.IPv6Mreq
			copy(mreq.Multiaddr[:], addr.IP.To16())
			mreq.Interface = uint32(intf.Index)

			err = syscall.SetsockoptIPv6Mreq(readSock, syscall.IPPROTO_IPV6, syscall.IPV6_ADD_MEMBERSHIP, &mreq)
		} else {
			var mreq syscall.IPMreq
			copy(mreq.Multiaddr[:], addr.IP.To4())
			err = setIPMreqInterface(&mreq, &intf)
			if err != nil {
				continue
			}

			err = syscall.SetsockoptIPMreq(readSock, syscall.IPPROTO_IP, syscall.IP_ADD_MEMBERSHIP, &mreq)
		}

		if err != nil {
			continue
		}

		enabledInterfaces = append(enabledInterfaces, &intf)
	}

	if enabledInterfaces == nil {
		syscall.Close(readSock) //nolint:errcheck
		return nil, fmt.Errorf("no multicast-capable interfaces found")
	}

	var writeFiles []*os.File
	var writeConns []net.PacketConn

	if !readOnly {
		writeSocks := make([]int, len(enabledInterfaces))

		for i, intf := range enabledInterfaces {
			var writeSock int
			writeSock, err = syscall.Socket(gi.Family, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
			if err != nil {
				for j := range i {
					syscall.Close(writeSocks[j]) //nolint:errcheck
				}
				syscall.Close(readSock) //nolint:errcheck
				return nil, err
			}

			err = syscall.SetsockoptInt(writeSock, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			if err != nil {
				syscall.Close(writeSock) //nolint:errcheck
				for j := range i {
					syscall.Close(writeSocks[j]) //nolint:errcheck
				}
				syscall.Close(readSock) //nolint:errcheck
				return nil, err
			}

			if gi.Family == syscall.AF_INET6 {
				lsa6.Port = addr.Port
				copy(lsa6.Addr[:], addr.IP.To16())
				err = syscall.Bind(writeSock, &lsa6)
				if err != nil {
					goto close_sockets
				}

				err = syscall.SetsockoptInt(writeSock, syscall.IPPROTO_IPV6, syscall.IPV6_MULTICAST_IF, intf.Index)
			} else {
				lsa4.Port = addr.Port
				copy(lsa4.Addr[:], addr.IP.To4())
				err = syscall.Bind(writeSock, &lsa4)
				if err != nil {
					goto close_sockets
				}

				var mreqn syscall.IPMreqn
				mreqn.Ifindex = int32(intf.Index)

				err = syscall.SetsockoptIPMreqn(writeSock, syscall.IPPROTO_IP, syscall.IP_MULTICAST_IF, &mreqn)
			}

			if err != nil {
				syscall.Close(writeSock) //nolint:errcheck
				for j := range i {
					syscall.Close(writeSocks[j]) //nolint:errcheck
				}
				syscall.Close(readSock) //nolint:errcheck
				return nil, err
			}

			err = syscall.SetsockoptInt(writeSock, gi.IPProto, gi.MulticastHops, multicastTTL)

		close_sockets:
			if err != nil {
				syscall.Close(writeSock) //nolint:errcheck
				for j := range i {
					syscall.Close(writeSocks[j]) //nolint:errcheck
				}
				syscall.Close(readSock) //nolint:errcheck
				return nil, err
			}

			writeSocks[i] = writeSock
		}

		writeFiles = make([]*os.File, len(writeSocks))
		writeConns = make([]net.PacketConn, len(writeSocks))

		for i, writeSock := range writeSocks {
			writeFiles[i] = os.NewFile(uintptr(writeSock), "")
			writeConns[i], _ = net.FilePacketConn(writeFiles[i])
		}
	}

	readFile := os.NewFile(uintptr(readSock), "")
	readConn, _ := net.FilePacketConn(readFile)

	return &multiConn{
		addr:       addr,
		readFile:   readFile,
		readConn:   readConn,
		writeFiles: writeFiles,
		writeConns: writeConns,
	}, nil
}

// Close implements Conn.
func (c *multiConn) Close() error {
	for i, writeConn := range c.writeConns {
		writeConn.Close()
		c.writeFiles[i].Close()
	}
	c.readConn.Close()
	c.readFile.Close()
	return nil
}

// SetReadBuffer implements Conn.
func (c *multiConn) SetReadBuffer(bytes int) error {
	return syscall.SetsockoptInt(int(c.readFile.Fd()), syscall.SOL_SOCKET, syscall.SO_RCVBUF, bytes)
}

// SyscallConn implements Conn.
func (c *multiConn) SyscallConn() (syscall.RawConn, error) {
	return &rawConn{fd: c.readFile.Fd()}, nil
}

// LocalAddr implements Conn.
func (c *multiConn) LocalAddr() net.Addr {
	return c.readConn.LocalAddr()
}

// SetDeadline implements Conn.
func (c *multiConn) SetDeadline(_ time.Time) error {
	panic("unimplemented")
}

// SetReadDeadline implements Conn.
func (c *multiConn) SetReadDeadline(t time.Time) error {
	return c.readConn.SetReadDeadline(t)
}

// SetWriteDeadline implements Conn.
func (c *multiConn) SetWriteDeadline(t time.Time) error {
	var err error
	for _, c := range c.writeConns {
		err2 := c.SetWriteDeadline(t)
		if err == nil {
			err = err2
		}
	}
	return err
}

// WriteTo implements Conn.
func (c *multiConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	var n int
	var err error
	for _, c := range c.writeConns {
		var err2 error
		n, err2 = c.WriteTo(b, addr)
		if err == nil {
			err = err2
		}
	}
	return n, err
}

// ReadFrom implements Conn.
func (c *multiConn) ReadFrom(b []byte) (int, net.Addr, error) {
	return c.readConn.ReadFrom(b)
}
