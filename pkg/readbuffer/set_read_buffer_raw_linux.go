package readbuffer

import (
	"fmt"
	"syscall"
)

func setReadBufferRawInControl(fd uintptr, size int) error {
	err := syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF, size)
	if err != nil {
		return err
	}

	v, err := syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_RCVBUF)
	if err != nil {
		return err
	}

	v /= 2 // Linux doubles the value set with SO_RCVBUF

	if v != size {
		return fmt.Errorf("unable to set UDP read buffer size to %d, got %d, check that the operating system allows that",
			size, v)
	}

	return nil
}
