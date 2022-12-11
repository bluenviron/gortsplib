package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/pkg/mpeg4audio"
)

func TestTrackMPEG4AudioAttributes(t *testing.T) {
	track := &TrackMPEG4Audio{
		PayloadType: 96,
		Config: &mpeg4audio.Config{
			Type:         mpeg4audio.ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}
	require.Equal(t, "MPEG4-audio", track.String())
	require.Equal(t, 48000, track.ClockRate())
}

func TestTrackMPEG4AudioClone(t *testing.T) {
	track := &TrackMPEG4Audio{
		PayloadType: 96,
		Config: &mpeg4audio.Config{
			Type:         mpeg4audio.ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackMPEG4AudioMediaDescription(t *testing.T) {
	track := &TrackMPEG4Audio{
		PayloadType: 96,
		Config: &mpeg4audio.Config{
			Type:         mpeg4audio.ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "mpeg4-generic/48000/2", rtpmap)
	require.Equal(t, "profile-level-id=1; mode=AAC-hbr; sizelength=13;"+
		" indexlength=3; indexdeltalength=3; config=1190", fmtp)
}
