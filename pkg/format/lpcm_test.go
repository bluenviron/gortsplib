package format

import (
	"testing"

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
}

func TestTracLPCMClone(t *testing.T) {
	format := &LPCM{
		PayloadTyp:   96,
		BitDepth:     16,
		SampleRate:   48000,
		ChannelCount: 2,
	}

	clone := format.Clone()
	require.NotSame(t, format, clone)
	require.Equal(t, format, clone)
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
