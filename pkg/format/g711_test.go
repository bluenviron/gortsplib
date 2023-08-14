package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestG711Attributes(t *testing.T) {
	format := &G711{}
	require.Equal(t, "G711", format.Codec())
	require.Equal(t, 8000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))

	format = &G711{
		MULaw: true,
	}
	require.Equal(t, "G711", format.Codec())
	require.Equal(t, 8000, format.ClockRate())
}

func TestG711DecEncoder(t *testing.T) {
	format := &G711{}

	enc, err := format.CreateEncoder()
	require.NoError(t, err)

	pkt, err := enc.Encode([]byte{0x01, 0x02, 0x03, 0x04})
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkt.PayloadType)

	dec, err := format.CreateDecoder()
	require.NoError(t, err)

	byts, err := dec.Decode(pkt)
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, byts)
}
