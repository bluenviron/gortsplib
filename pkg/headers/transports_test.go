package headers

import (
	"testing"

	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/stretchr/testify/require"
)

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
				Delivery:    ptrOf(TransportDeliveryUnicast),
				ClientPorts: &[2]int{3456, 3457},
				Mode:        ptrOf(TransportModePlay),
			},
			Transport{
				Protocol:       TransportProtocolTCP,
				Delivery:       ptrOf(TransportDeliveryUnicast),
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

	f.Fuzz(func(_ *testing.T, b string) {
		var h Transports
		err := h.Unmarshal(base.HeaderValue{b})
		if err != nil {
			return
		}

		h.Marshal()
	})
}

func TestTransportsAdditionalErrors(t *testing.T) {
	func() {
		var h Transports
		err := h.Unmarshal(base.HeaderValue{})
		require.Error(t, err)
	}()

	func() {
		var h Transports
		err := h.Unmarshal(base.HeaderValue{"a", "b"})
		require.Error(t, err)
	}()
}
