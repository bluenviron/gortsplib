package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackVorbisAttributes(t *testing.T) {
	track := &TrackVorbis{
		PayloadType:   96,
		SampleRate:    48000,
		ChannelCount:  2,
		Configuration: []byte{0x01, 0x02, 0x03, 0x04},
	}
	require.Equal(t, "Vorbis", track.String())
	require.Equal(t, 48000, track.ClockRate())
}

func TestTracVorbisClone(t *testing.T) {
	track := &TrackVorbis{
		PayloadType:   96,
		SampleRate:    48000,
		ChannelCount:  2,
		Configuration: []byte{0x01, 0x02, 0x03, 0x04},
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackVorbisMediaDescription(t *testing.T) {
	track := &TrackVorbis{
		PayloadType:   96,
		SampleRate:    48000,
		ChannelCount:  2,
		Configuration: []byte{0x01, 0x02, 0x03, 0x04},
	}

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "VORBIS/48000/2", rtpmap)
	require.Equal(t, "configuration=AQIDBA==", fmtp)
}
