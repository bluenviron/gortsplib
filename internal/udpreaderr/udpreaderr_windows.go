// Package udpreaderr contains UDP read error helpers.
package udpreaderr

import (
	"errors"

	"golang.org/x/sys/windows"
)

// IsPacketTooBig reports whether an error is caused by a truncated UDP packet.
func IsPacketTooBig(err error) bool {
	// Windows reports truncated UDP packets as WSAEMSGSIZE.
	return errors.Is(err, windows.WSAEMSGSIZE)
}
