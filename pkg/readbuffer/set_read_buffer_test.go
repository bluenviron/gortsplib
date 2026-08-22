package readbuffer_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/readbuffer"
)

func TestSetReadBuffer(t *testing.T) {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:3456")
	require.NoError(t, err)

	pc, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)
	defer pc.Close() //nolint:errcheck

	err = readbuffer.SetReadBuffer(pc, 10000)
	require.NoError(t, err)
}
