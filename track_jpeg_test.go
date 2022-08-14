package gortsplib //nolint:dupl

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackJPEGAttributes(t *testing.T) {
	track := &TrackJPEG{}
	require.Equal(t, 90000, track.ClockRate())
	require.Equal(t, "", track.GetControl())
}

func TestTrackJPEGClone(t *testing.T) {
	track := &TrackJPEG{}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackJPEGMediaDescription(t *testing.T) {
	track := &TrackJPEG{}

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "video",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"26"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "26 JPEG/90000",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
