package track

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpusAttributes(t *testing.T) {
	track := &Opus{
		PayloadTyp:   96,
		SampleRate:   48000,
		ChannelCount: 2,
	}
	require.Equal(t, "Opus", track.String())
	require.Equal(t, 48000, track.ClockRate())
	require.Equal(t, uint8(96), track.PayloadType())
}

func TestTracOpusClone(t *testing.T) {
	track := &Opus{
		PayloadTyp:   96,
		SampleRate:   48000,
		ChannelCount: 2,
	}

	clone := track.Clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestOpusMediaDescription(t *testing.T) {
	track := &Opus{
		PayloadTyp:   96,
		SampleRate:   48000,
		ChannelCount: 2,
	}

	rtpmap, fmtp := track.Marshal()
	require.Equal(t, "opus/48000/2", rtpmap)
	require.Equal(t, "sprop-stereo=1", fmtp)
}
