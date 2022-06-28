package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTracVP8Clone(t *testing.T) {
	maxFR := 123
	maxFS := 456
	track := &TrackVP8{
		PayloadType: 96,
		MaxFR:       &maxFR,
		MaxFS:       &maxFS,
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackVP8MediaDescription(t *testing.T) {
	maxFR := 123
	maxFS := 456
	track := &TrackVP8{
		PayloadType: 96,
		MaxFR:       &maxFR,
		MaxFS:       &maxFS,
	}

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "video",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"96"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "96 VP8/90000",
			},
			{
				Key:   "fmtp",
				Value: "96 max-fr=123;max-fs=456",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
