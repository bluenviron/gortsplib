package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesHeaderTransport = []struct {
	name string
	vin  HeaderValue
	vout HeaderValue
	h    *HeaderTransport
}{
	{
		"udp unicast play request",
		HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode="PLAY"`},
		HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode=play`},
		&HeaderTransport{
			Protocol: StreamProtocolUDP,
			Cast: func() *StreamCast {
				ret := StreamUnicast
				return &ret
			}(),
			ClientPorts: &[2]int{3456, 3457},
			Mode: func() *string {
				ret := "play"
				return &ret
			}(),
		},
	},
	{
		"udp unicast play response",
		HeaderValue{`RTP/AVP/UDP;unicast;client_port=3056-3057;server_port=5000-5001`},
		HeaderValue{`RTP/AVP;unicast;client_port=3056-3057;server_port=5000-5001`},
		&HeaderTransport{
			Protocol: StreamProtocolUDP,
			Cast: func() *StreamCast {
				ret := StreamUnicast
				return &ret
			}(),
			ClientPorts: &[2]int{3056, 3057},
			ServerPorts: &[2]int{5000, 5001},
		},
	},
	{
		"udp multicast play request / response",
		HeaderValue{`RTP/AVP;multicast;destination=225.219.201.15;port=7000-7001;ttl=127`},
		HeaderValue{`RTP/AVP;multicast`},
		&HeaderTransport{
			Protocol: StreamProtocolUDP,
			Cast: func() *StreamCast {
				ret := StreamMulticast
				return &ret
			}(),
		},
	},
	{
		"tcp play request / response",
		HeaderValue{`RTP/AVP/TCP;interleaved=0-1`},
		HeaderValue{`RTP/AVP/TCP;interleaved=0-1`},
		&HeaderTransport{
			Protocol:       StreamProtocolTCP,
			InterleavedIds: &[2]int{0, 1},
		},
	},
}

func TestHeaderTransportRead(t *testing.T) {
	for _, c := range casesHeaderTransport {
		t.Run(c.name, func(t *testing.T) {
			req, err := ReadHeaderTransport(c.vin)
			require.NoError(t, err)
			require.Equal(t, c.h, req)
		})
	}
}

func TestHeaderTransportWrite(t *testing.T) {
	for _, c := range casesHeaderTransport {
		t.Run(c.name, func(t *testing.T) {
			req := c.h.Write()
			require.Equal(t, c.vout, req)
		})
	}
}
