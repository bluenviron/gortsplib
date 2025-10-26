package readbuffer

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetReadBuffer(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:3456")
	require.NoError(t, err)

	pc, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)
	defer pc.Close() //nolint:errcheck

	err = SetReadBuffer(pc, 10000)
	require.NoError(t, err)
}
