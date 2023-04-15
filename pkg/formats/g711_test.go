package formats

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestG711Attributes(t *testing.T) {
	format := &G711{}
	require.Equal(t, "G711", format.String())
	require.Equal(t, 8000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))

	format = &G711{
		MULaw: true,
	}
	require.Equal(t, "G711", format.String())
	require.Equal(t, 8000, format.ClockRate())
}

func TestG711DecEncoder(t *testing.T) {
	format := &G711{}

	enc := format.CreateEncoder()
	pkt, err := enc.Encode([]byte{0x01, 0x02, 0x03, 0x04}, 0)
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkt.PayloadType)

	dec := format.CreateDecoder()
	byts, _, err := dec.Decode(pkt)
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, byts)
}
