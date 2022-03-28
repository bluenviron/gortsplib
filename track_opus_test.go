package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackOpusNew(t *testing.T) {
	track, err := NewTrackOpus(96, 48000, 2)
	require.NoError(t, err)
	require.Equal(t, "", track.GetControl())
	require.Equal(t, 48000, track.ClockRate())
	require.Equal(t, 2, track.ChannelCount())
}

func TestTracOpusClone(t *testing.T) {
	track, err := NewTrackOpus(96, 96000, 4)
	require.NoError(t, err)

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackOpusMediaDescription(t *testing.T) {
	track, err := NewTrackOpus(96, 48000, 2)
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
				Value: "96 opus/48000/2",
			},
			{
				Key:   "fmtp",
				Value: "96 sprop-stereo=1",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
