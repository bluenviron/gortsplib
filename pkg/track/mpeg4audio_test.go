package track

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/v2/pkg/mpeg4audio"
)

func TestMPEG4AudioAttributes(t *testing.T) {
	track := &MPEG4Audio{
		PayloadTyp: 96,
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
	require.Equal(t, uint8(96), track.PayloadType())
}

func TestMPEG4AudioClone(t *testing.T) {
	track := &MPEG4Audio{
		PayloadTyp: 96,
		Config: &mpeg4audio.Config{
			Type:         mpeg4audio.ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}

	clone := track.Clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestMPEG4AudioMediaDescription(t *testing.T) {
	track := &MPEG4Audio{
		PayloadTyp: 96,
		Config: &mpeg4audio.Config{
			Type:         mpeg4audio.ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}

	rtpmap, fmtp := track.Marshal()
	require.Equal(t, "mpeg4-generic/48000/2", rtpmap)
	require.Equal(t, "profile-level-id=1; mode=AAC-hbr; sizelength=13;"+
		" indexlength=3; indexdeltalength=3; config=1190", fmtp)
}
