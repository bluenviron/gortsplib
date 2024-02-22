package headers

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v4/pkg/base"
)

func ipPtr(v net.IP) *net.IP {
	return &v
}

func deliveryPtr(v TransportDelivery) *TransportDelivery {
	return &v
}

func transportModePtr(v TransportMode) *TransportMode {
	return &v
}

var casesTransport = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    Transport
}{
	{
		"udp unicast play request",
		base.HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode="PLAY"`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode=play`},
		Transport{
			Protocol:    TransportProtocolUDP,
			Delivery:    deliveryPtr(TransportDeliveryUnicast),
			ClientPorts: &[2]int{3456, 3457},
			Mode:        transportModePtr(TransportModePlay),
		},
	},
	{
		"udp unicast play response",
		base.HeaderValue{`RTP/AVP/UDP;unicast;client_port=3056-3057;server_port=5000-5001`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=3056-3057;server_port=5000-5001`},
		Transport{
			Protocol:    TransportProtocolUDP,
			Delivery:    deliveryPtr(TransportDeliveryUnicast),
			ClientPorts: &[2]int{3056, 3057},
			ServerPorts: &[2]int{5000, 5001},
		},
	},
	{
		"udp multicast play request / response",
		base.HeaderValue{`RTP/AVP;multicast;destination=225.219.201.15;port=7000-7001;ttl=127`},
		base.HeaderValue{`RTP/AVP;multicast;destination=225.219.201.15;port=7000-7001;ttl=127`},
		Transport{
			Protocol:    TransportProtocolUDP,
			Delivery:    deliveryPtr(TransportDeliveryMulticast),
			Destination: ipPtr(net.ParseIP("225.219.201.15")),
			TTL:         uintPtr(127),
			Ports:       &[2]int{7000, 7001},
		},
	},
	{
		"tcp play request / response",
		base.HeaderValue{`RTP/AVP/TCP;interleaved=0-1`},
		base.HeaderValue{`RTP/AVP/TCP;interleaved=0-1`},
		Transport{
			Protocol:       TransportProtocolTCP,
			InterleavedIDs: &[2]int{0, 1},
		},
	},
	{
		"udp unicast play response with a single port and ssrc",
		base.HeaderValue{`RTP/AVP/UDP;unicast;server_port=8052;client_port=14186;ssrc=0B6020AD;mode=PLAY`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=14186-14187;server_port=8052-8053;ssrc=0B6020AD;mode=play`},
		Transport{
			Protocol:    TransportProtocolUDP,
			Delivery:    deliveryPtr(TransportDeliveryUnicast),
			Mode:        transportModePtr(TransportModePlay),
			ClientPorts: &[2]int{14186, 14187},
			ServerPorts: &[2]int{8052, 8053},
			SSRC:        uint32Ptr(0x0B6020AD),
		},
	},
	{
		"udp record response with receive",
		base.HeaderValue{`RTP/AVP/UDP;unicast;mode=receive;source=127.0.0.1;client_port=14186-14187;server_port=5000-5001`},
		base.HeaderValue{`RTP/AVP;unicast;source=127.0.0.1;client_port=14186-14187;server_port=5000-5001;mode=record`},
		Transport{
			Protocol:    TransportProtocolUDP,
			Delivery:    deliveryPtr(TransportDeliveryUnicast),
			Mode:        transportModePtr(TransportModeRecord),
			ClientPorts: &[2]int{14186, 14187},
			ServerPorts: &[2]int{5000, 5001},
			Source:      ipPtr(net.ParseIP("127.0.0.1")),
		},
	},
	{
		"unsorted udp unicast play request headers",
		base.HeaderValue{`client_port=3456-3457;RTP/AVP;mode="PLAY";unicast`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode=play`},
		Transport{
			Protocol:    TransportProtocolUDP,
			Delivery:    deliveryPtr(TransportDeliveryUnicast),
			ClientPorts: &[2]int{3456, 3457},
			Mode:        transportModePtr(TransportModePlay),
		},
	},
	{
		"ssrc odd",
		base.HeaderValue{`RTP/AVP/UDP;unicast;client_port=14186;server_port=8052;ssrc=4317f;mode=play`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=14186-14187;server_port=8052-8053;ssrc=0004317F;mode=play`},
		Transport{
			Protocol:    TransportProtocolUDP,
			Delivery:    deliveryPtr(TransportDeliveryUnicast),
			Mode:        transportModePtr(TransportModePlay),
			ClientPorts: &[2]int{14186, 14187},
			ServerPorts: &[2]int{8052, 8053},
			SSRC:        uint32Ptr(0x04317f),
		},
	},
	{
		"hikvision ssrc with initial spaces",
		base.HeaderValue{`RTP/AVP/UDP;unicast;client_port=14186;server_port=8052;ssrc= 4317f;mode=play`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=14186-14187;server_port=8052-8053;ssrc=0004317F;mode=play`},
		Transport{
			Protocol:    TransportProtocolUDP,
			Delivery:    deliveryPtr(TransportDeliveryUnicast),
			Mode:        transportModePtr(TransportModePlay),
			ClientPorts: &[2]int{14186, 14187},
			ServerPorts: &[2]int{8052, 8053},
			SSRC:        uint32Ptr(0x04317f),
		},
	},
	{
		"dahua rtsp server ssrc with initial spaces",
		base.HeaderValue{`RTP/AVP/TCP;unicast;interleaved=0-1;ssrc=     D93FF`},
		base.HeaderValue{`RTP/AVP/TCP;unicast;interleaved=0-1;ssrc=000D93FF`},
		Transport{
			Protocol:       TransportProtocolTCP,
			Delivery:       deliveryPtr(TransportDeliveryUnicast),
			InterleavedIDs: &[2]int{0, 1},
			SSRC:           uint32Ptr(0xD93FF),
		},
	},
	{
		"empty source",
		base.HeaderValue{`RTP/AVP/UDP;unicast;source=;client_port=32560-32561;server_port=3046-3047;ssrc=45dcb578`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=32560-32561;server_port=3046-3047;ssrc=45DCB578`},
		Transport{
			Protocol:    TransportProtocolUDP,
			Delivery:    deliveryPtr(TransportDeliveryUnicast),
			SSRC:        uint32Ptr(0x45dcb578),
			ClientPorts: &[2]int{32560, 32561},
			ServerPorts: &[2]int{3046, 3047},
		},
	},
	{
		"invalid ssrc",
		base.HeaderValue{`RTP/AVP;unicast;client_port=14236;source=172.16.8.2;server_port=56002;ssrc=1449463210`},
		base.HeaderValue{`RTP/AVP;unicast;source=172.16.8.2;client_port=14236-14237;server_port=56002-56003`},
		Transport{
			Protocol:    TransportProtocolUDP,
			Delivery:    deliveryPtr(TransportDeliveryUnicast),
			Source:      ipPtr(net.ParseIP("172.16.8.2")),
			ClientPorts: &[2]int{14236, 14237},
			ServerPorts: &[2]int{56002, 56003},
		},
	},
}

func TestTransportUnmarshal(t *testing.T) {
	for _, ca := range casesTransport {
		t.Run(ca.name, func(t *testing.T) {
			var h Transport
			err := h.Unmarshal(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestTransportMarshal(t *testing.T) {
	for _, ca := range casesTransport {
		t.Run(ca.name, func(t *testing.T) {
			req := ca.h.Marshal()
			require.Equal(t, ca.vout, req)
		})
	}
}

var casesTransports = []struct {
	name string
	vin  base.HeaderValue
	vout base.HeaderValue
	h    Transports
}{
	{
		"a",
		base.HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode="PLAY", RTP/AVP/TCP;unicast;interleaved=0-1`},
		base.HeaderValue{`RTP/AVP;unicast;client_port=3456-3457;mode=play,RTP/AVP/TCP;unicast;interleaved=0-1`},
		Transports{
			{
				Protocol:    TransportProtocolUDP,
				Delivery:    deliveryPtr(TransportDeliveryUnicast),
				ClientPorts: &[2]int{3456, 3457},
				Mode:        transportModePtr(TransportModePlay),
			},
			Transport{
				Protocol:       TransportProtocolTCP,
				Delivery:       deliveryPtr(TransportDeliveryUnicast),
				InterleavedIDs: &[2]int{0, 1},
			},
		},
	},
}

func TestTransportsUnmarshal(t *testing.T) {
	for _, ca := range casesTransports {
		t.Run(ca.name, func(t *testing.T) {
			var h Transports
			err := h.Unmarshal(ca.vin)
			require.NoError(t, err)
			require.Equal(t, ca.h, h)
		})
	}
}

func TestTransportsMarshal(t *testing.T) {
	for _, ca := range casesTransports {
		t.Run(ca.name, func(t *testing.T) {
			req := ca.h.Marshal()
			require.Equal(t, ca.vout, req)
		})
	}
}

func FuzzTransportsUnmarshal(f *testing.F) {
	for _, ca := range casesTransports {
		f.Add(ca.vin[0])
	}

	for _, ca := range casesTransport {
		f.Add(ca.vin[0])
	}

	f.Add("source=aa-14187")
	f.Add("destination=aa")
	f.Add("interleaved=")
	f.Add("ttl=")
	f.Add("port=")
	f.Add("client_port=")
	f.Add("server_port=")
	f.Add("mode=")

	f.Fuzz(func(t *testing.T, b string) {
		var h Transports
		h.Unmarshal(base.HeaderValue{b}) //nolint:errcheck
	})
}

func TestTransportAdditionalErrors(t *testing.T) {
	func() {
		var h Transport
		err := h.Unmarshal(base.HeaderValue{})
		require.Error(t, err)
	}()

	func() {
		var h Transport
		err := h.Unmarshal(base.HeaderValue{"a", "b"})
		require.Error(t, err)
	}()
}
