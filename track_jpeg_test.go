package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackJPEGNew(t *testing.T) {
	track := NewTrackJPEG()
	require.Equal(t, "", track.GetControl())
}

func TestTrackJPEGClone(t *testing.T) {
	track := NewTrackJPEG()

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackJPEGMediaDescription(t *testing.T) {
	track := NewTrackJPEG()

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
