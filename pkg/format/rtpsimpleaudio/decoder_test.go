package rtpsimpleaudio

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			var d Decoder
			err := d.Init()
			require.NoError(t, err)

			frame, err := d.Decode(ca.pkt)
			require.NoError(t, err)
			require.Equal(t, ca.frame, frame)
		})
	}
}

func TestDecodeErrorEmpty(t *testing.T) {
	var d Decoder
	err := d.Init()
	require.NoError(t, err)

	_, err = d.Decode(&rtp.Packet{
		Payload: []byte{},
	})
	require.EqualError(t, err, "payload is too short")
}
