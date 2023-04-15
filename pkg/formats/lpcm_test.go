package formats

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestLPCMAttributes(t *testing.T) {
	format := &LPCM{
		PayloadTyp:   96,
		BitDepth:     24,
		SampleRate:   44100,
		ChannelCount: 2,
	}
	require.Equal(t, "LPCM", format.String())
	require.Equal(t, 44100, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestLPCMDecEncoder(t *testing.T) {
	format := &LPCM{
		PayloadTyp:   96,
		BitDepth:     16,
		SampleRate:   96000,
		ChannelCount: 2,
	}

	enc := format.CreateEncoder()
	pkts, err := enc.Encode([]byte{0x01, 0x02, 0x03, 0x04}, 0)
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec := format.CreateDecoder()
	byts, _, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, byts)
}
