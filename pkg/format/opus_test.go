package format_test //nolint:revive

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
)

func TestOpusAttributes(t *testing.T) {
	format := &format.Opus{
		PayloadTyp:   96,
		ChannelCount: 2,
	}
	require.Equal(t, "Opus", format.Codec())
	require.Equal(t, 48000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestOpusDecEncoder(t *testing.T) {
	format := &format.Opus{}

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
