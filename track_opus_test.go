package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackOpusAttributes(t *testing.T) {
	track := &TrackOpus{
		PayloadType:  96,
		SampleRate:   48000,
		ChannelCount: 2,
	}
	require.Equal(t, "Opus", track.String())
	require.Equal(t, 48000, track.ClockRate())
}

func TestTracOpusClone(t *testing.T) {
	track := &TrackOpus{
		PayloadType:  96,
		SampleRate:   48000,
		ChannelCount: 2,
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackOpusMediaDescription(t *testing.T) {
	track := &TrackOpus{
		PayloadType:  96,
		SampleRate:   48000,
		ChannelCount: 2,
	}

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "opus/48000/2", rtpmap)
	require.Equal(t, "sprop-stereo=1", fmtp)
}
