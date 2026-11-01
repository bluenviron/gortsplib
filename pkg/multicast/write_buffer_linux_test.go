package multicast_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/multicast"
)

func TestWriteBuffer(t *testing.T) {
	for _, mode := range []string{"single", "multi"} {
		t.Run(mode, func(t *testing.T) {
			var conn multicast.Conn
			var err error
			if mode == "single" {
				interfaces, err2 := net.Interfaces()
				require.NoError(t, err2)
				var intf *net.Interface
				for _, candidate := range interfaces {
					if candidate.Flags&net.FlagMulticast != 0 {
						intf = &candidate
						break
					}
				}
				if intf == nil {
					t.Skip("no multicast-capable interfaces found")
				}
				conn, err = multicast.NewSingleConn(intf, "224.1.0.1:0", net.ListenPacket)
			} else {
				conn, err = multicast.NewMultiConn("224.1.0.1:0", false, net.ListenPacket)
			}
			require.NoError(t, err)
			defer conn.Close() //nolint:errcheck

			require.NoError(t, conn.SetWriteBuffer(10000))
			err = conn.SetWriteBuffer(-1)
			require.ErrorContains(t, err, "unable to set UDP write buffer size")
		})
	}
}
