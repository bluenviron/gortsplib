package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/base"
)

var casesHeaderTransport = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    *HeaderTransport
}{
	{
		"udp unicast play request",
		base.HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode="PLAY"`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode=play`},
		&HeaderTransport{
			Protocol: StreamProtocolUDP,
			Cast: func() *StreamCast {
				v := StreamUnicast
				return &v
			}(),
			ClientPorts: &[2]int{3456, 3457},
			Mode: func() *string {
				v := "play"
				return &v
			}(),
		},
	},
	{
		"udp unicast play response",
		base.HeaderValue{`RTP/AVP/UDP;unicast;client_port=3056-3057;server_port=5000-5001`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=3056-3057;server_port=5000-5001`},
		&HeaderTransport{
			Protocol: StreamProtocolUDP,
			Cast: func() *StreamCast {
				v := StreamUnicast
				return &v
			}(),
			ClientPorts: &[2]int{3056, 3057},
			ServerPorts: &[2]int{5000, 5001},
		},
	},
	{
		"udp multicast play request / response",
		base.HeaderValue{`RTP/AVP;multicast;destination=225.219.201.15;port=7000-7001;ttl=127`},
		base.HeaderValue{`RTP/AVP;multicast`},
		&HeaderTransport{
			Protocol: StreamProtocolUDP,
			Cast: func() *StreamCast {
				v := StreamMulticast
				return &v
			}(),
			Destination: func() *string {
				v := "225.219.201.15"
				return &v
			}(),
			TTL: func() *uint {
				v := uint(127)
				return &v
			}(),
			Ports: &[2]int{7000, 7001},
		},
	},
	{
		"tcp play request / response",
		base.HeaderValue{`RTP/AVP/TCP;interleaved=0-1`},
		base.HeaderValue{`RTP/AVP/TCP;interleaved=0-1`},
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
