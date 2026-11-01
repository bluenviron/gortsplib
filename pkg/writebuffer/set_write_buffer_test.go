package writebuffer_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/writebuffer"
)

func TestSetWriteBuffer(t *testing.T) {
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	require.NoError(t, err)
	defer pc.Close() //nolint:errcheck

	err = writebuffer.SetWriteBuffer(pc, 10000)
	require.NoError(t, err)
}

func TestSetWriteBufferRaw(t *testing.T) {
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	require.NoError(t, err)
	defer pc.Close() //nolint:errcheck

	rawConn, err := pc.SyscallConn()
	require.NoError(t, err)

	err = writebuffer.SetWriteBufferRaw(rawConn, 10000)
	require.NoError(t, err)
}

func TestSetWriteBufferClosed(t *testing.T) {
	pc, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	require.NoError(t, err)
	require.NoError(t, pc.Close())

	err = writebuffer.SetWriteBuffer(pc, 10000)
	require.Error(t, err)
}
