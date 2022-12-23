package format

import (
	"fmt"
	"strconv"
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
	require.Equal(t, uint8(96), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestLPCMMediaDescription(t *testing.T) {
	for _, ca := range []int{8, 16, 24} {
		t.Run(strconv.FormatInt(int64(ca), 10), func(t *testing.T) {
			format := &LPCM{
				PayloadTyp:   96,
				BitDepth:     ca,
				SampleRate:   96000,
				ChannelCount: 2,
			}

			rtpmap, fmtp := format.Marshal()
			require.Equal(t, fmt.Sprintf("L%d/96000/2", ca), rtpmap)
			require.Equal(t, "", fmtp)
		})
	}
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
