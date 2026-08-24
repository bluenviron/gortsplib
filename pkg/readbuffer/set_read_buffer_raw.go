package readbuffer

import "syscall"

// SetReadBufferRaw sets the read buffer size of the raw UDP connection and checks that it was set correctly.
func SetReadBufferRaw(rc syscall.RawConn, size int) error {
	var err2 error

	err := rc.Control(func(fd uintptr) {
		err2 = setReadBufferRawInControl(fd, size)
	})
	if err != nil {
		return err
	}

	return err2
}
