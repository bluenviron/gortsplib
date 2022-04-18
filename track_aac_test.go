package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackAACNew(t *testing.T) {
	track, err := NewTrackAAC(96, 2, 48000, 4, []byte{0x01, 0x02}, 13, 3, 3)
	require.NoError(t, err)
	require.Equal(t, "", track.GetControl())
	require.Equal(t, 2, track.Type())
	require.Equal(t, 48000, track.ClockRate())
	require.Equal(t, 4, track.ChannelCount())
	require.Equal(t, []byte{0x01, 0x02}, track.AOTSpecificConfig())
	require.Equal(t, 13, track.SizeLength())
	require.Equal(t, 3, track.IndexLength())
	require.Equal(t, 3, track.IndexDeltaLength())
}

func TestTrackAACNewErrors(t *testing.T) {
	_, err := NewTrackAAC(96, 2, 48000, 10, nil, 13, 3, 3)
	require.EqualError(t, err, "invalid configuration: invalid channel count (10)")
}

func TestTrackAACClone(t *testing.T) {
	track, err := NewTrackAAC(96, 2, 48000, 2, []byte{0x01, 0x02}, 13, 3, 3)
	require.NoError(t, err)

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackAACMediaDescription(t *testing.T) {
	track, err := NewTrackAAC(96, 2, 48000, 2, nil, 13, 3, 3)
	require.NoError(t, err)

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"96"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "96 mpeg4-generic/48000/2",
			},
			{
				Key:   "fmtp",
				Value: "96 profile-level-id=1; mode=AAC-hbr; sizelength=13; indexlength=3; indexdeltalength=3; config=1190",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
