package headers

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/base"
)

var casesTransport = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    *Transport
}{
	{
		"udp unicast play request",
		base.HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode="PLAY"`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode=play`},
		&Transport{
			Protocol: base.StreamProtocolUDP,
			Cast: func() *base.StreamCast {
				v := base.StreamUnicast
				return &v
			}(),
			ClientPorts: &[2]int{3456, 3457},
			Mode: func() *TransportMode {
				v := TransportModePlay
				return &v
			}(),
		},
	},
	{
		"udp unicast play response",
		base.HeaderValue{`RTP/AVP/UDP;unicast;client_port=3056-3057;server_port=5000-5001`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=3056-3057;server_port=5000-5001`},
		&Transport{
			Protocol: base.StreamProtocolUDP,
			Cast: func() *base.StreamCast {
				v := base.StreamUnicast
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
		&Transport{
			Protocol: base.StreamProtocolUDP,
			Cast: func() *base.StreamCast {
				v := base.StreamMulticast
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
		&Transport{
			Protocol:       base.StreamProtocolTCP,
			InterleavedIds: &[2]int{0, 1},
		},
	},
	{
		"udp record response with receive",
		base.HeaderValue{`RTP/AVP/UDP;unicast;mode=receive;source=localhost;client_port=14186-14187;server_port=5000-5001`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=14186-14187;server_port=5000-5001;mode=record`},
		&Transport{
			Protocol: base.StreamProtocolUDP,
			Cast: func() *base.StreamCast {
				v := base.StreamUnicast
				return &v
			}(),
			Mode: func() *TransportMode {
				v := TransportModeRecord
				return &v
			}(),
			ClientPorts: &[2]int{14186, 14187},
			ServerPorts: &[2]int{5000, 5001},
		},
	},
}

func TestTransportRead(t *testing.T) {
	for _, c := range casesTransport {
		t.Run(c.name, func(t *testing.T) {
			req, err := ReadTransport(c.vin)
			require.NoError(t, err)
			require.Equal(t, c.h, req)
		})
	}
}

func TestTransportWrite(t *testing.T) {
	for _, c := range casesTransport {
		t.Run(c.name, func(t *testing.T) {
			req := c.h.Write()
			require.Equal(t, c.vout, req)
		})
	}
}
