package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestG711Attributes(t *testing.T) {
	for _, ca := range []struct {
		name      string
		format    *G711
		clockRate int
	}{
		{
			"pcma 8khz",
			&G711{
				PayloadTyp:   8,
				MULaw:        false,
				SampleRate:   8000,
				ChannelCount: 1,
			},
			8000,
		},
		{
			"pcmu 8khz",
			&G711{
				PayloadTyp:   0,
				MULaw:        true,
				SampleRate:   8000,
				ChannelCount: 1,
			},
			8000,
		},
		{
			"pcma 16khz",
			&G711{
				PayloadTyp:   96,
				MULaw:        true,
				SampleRate:   16000,
				ChannelCount: 1,
			},
			16000,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			require.Equal(t, "G711", ca.format.Codec())
			require.Equal(t, ca.clockRate, ca.format.ClockRate())
			require.Equal(t, true, ca.format.PTSEqualsDTS(&rtp.Packet{}))
		})
	}
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
