//go:build !windows

// Package udpreaderr contains UDP read error helpers.
package udpreaderr

// IsPacketTooBig reports whether an error is caused by a truncated UDP packet.
func IsPacketTooBig(_ error) bool {
	return false
}
