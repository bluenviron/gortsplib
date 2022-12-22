package format

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
	require.Equal(t, uint8(96), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestLPCMMediaDescription(t *testing.T) {
	format := &LPCM{
		PayloadTyp:   96,
		BitDepth:     24,
		SampleRate:   96000,
		ChannelCount: 2,
	}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "L24/96000/2", rtpmap)
	require.Equal(t, "", fmtp)
}
