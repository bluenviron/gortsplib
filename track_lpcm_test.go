package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackLPCMAttributes(t *testing.T) {
	track := &TrackLPCM{
		PayloadType:  96,
		BitDepth:     24,
		SampleRate:   44100,
		ChannelCount: 2,
	}
	require.Equal(t, 44100, track.ClockRate())
	require.Equal(t, "", track.GetControl())
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

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"96"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "96 L24/96000/2",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
