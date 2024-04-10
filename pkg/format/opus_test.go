package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestOpusAttributes(t *testing.T) {
	format := &Opus{
		PayloadTyp: 96,
		IsStereo:   true,
	}
	require.Equal(t, "Opus", format.Codec())
	require.Equal(t, 48000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestOpusDecEncoder(t *testing.T) {
	format := &Opus{}

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
