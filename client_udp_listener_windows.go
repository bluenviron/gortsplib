//go:build windows

package gortsplib

import (
	"fmt"
	"syscall"
)

func setAndVerifyReadBufferSize(pc packetConn, v int) error {
	rawConn, err := pc.SyscallConn()
	if err != nil {
		panic(err)
	}

	var err2 error

	err = rawConn.Control(func(fd uintptr) {
		// On Windows, syscall.SetsockoptInt expects syscall.Handle
		handle := syscall.Handle(fd)
		err2 = syscall.SetsockoptInt(handle, syscall.SOL_SOCKET, syscall.SO_RCVBUF, v)
		if err2 != nil {
			return
		}

		var v2 int
		v2, err2 = syscall.GetsockoptInt(handle, syscall.SOL_SOCKET, syscall.SO_RCVBUF)
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
