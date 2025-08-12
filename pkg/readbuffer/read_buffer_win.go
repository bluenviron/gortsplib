//go:build windows

package readbuffer

import "syscall"

// ReadBuffer returns the read buffer size.
func ReadBuffer(pc PacketConn) (int, error) {
	rawConn, err := pc.SyscallConn()
	if err != nil {
		panic(err)
	}

	var v int
	var err2 error

	err = rawConn.Control(func(fd uintptr) {
		v, err2 = syscall.GetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
	})
	if err != nil {
		return 0, err
	}

	if err2 != nil {
		return 0, err2
	}

	return v, nil
}
