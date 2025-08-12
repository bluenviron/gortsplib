//go:build !linux && !windows

package readbuffer

import "fmt"

// ReadBuffer returns the read buffer size.
func ReadBuffer(pc PacketConn) (int, error) {
	return 0, fmt.Errorf("read buffer size is unimplemented on the current operating system")
}
