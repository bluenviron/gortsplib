package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackLPCMAttributes(t *testing.T) {
	track := &TrackLPCM{
		PayloadType:  96,
		BitDepth:     24,
		SampleRate:   44100,
		ChannelCount: 2,
	}
	require.Equal(t, "LPCM", track.String())
	require.Equal(t, 44100, track.ClockRate())
}

func TestTracLPCMClone(t *testing.T) {
	track := &TrackLPCM{
		PayloadType:  96,
		BitDepth:     16,
		SampleRate:   48000,
		ChannelCount: 2,
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackLPCMMediaDescription(t *testing.T) {
	track := &TrackLPCM{
		PayloadType:  96,
		BitDepth:     24,
		SampleRate:   96000,
		ChannelCount: 2,
	}

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "L24/96000/2", rtpmap)
	require.Equal(t, "", fmtp)
}
