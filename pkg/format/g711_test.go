package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestG711Attributes(t *testing.T) {
	t.Run("pcma", func(t *testing.T) {
		format := &G711{
			PayloadTyp:   8,
			MULaw:        false,
			SampleRate:   8000,
			ChannelCount: 1,
		}
		require.Equal(t, "G711", format.Codec())
		require.Equal(t, 8000, format.ClockRate())
		require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
	})

	t.Run("pcmu", func(t *testing.T) {
		format := &G711{
			PayloadTyp:   0,
			MULaw:        true,
			SampleRate:   8000,
			ChannelCount: 1,
		}
		require.Equal(t, "G711", format.Codec())
		require.Equal(t, 8000, format.ClockRate())
	})
}

func TestG711DecEncoder(t *testing.T) {
	format := &G711{
		PayloadTyp:   8,
		MULaw:        false,
		SampleRate:   8000,
		ChannelCount: 1,
	}

	enc, err := format.CreateEncoder()
	require.NoError(t, err)

	pkts, err := enc.Encode([]byte{0x01, 0x02, 0x03, 0x04})
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec, err := format.CreateDecoder()
	require.NoError(t, err)

	byts, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, byts)
}
