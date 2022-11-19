package gortsplib //nolint:dupl

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackG722Attributes(t *testing.T) {
	track := &TrackG722{}
	require.Equal(t, "G722", track.String())
	require.Equal(t, 8000, track.ClockRate())
	require.Equal(t, "", track.GetControl())
}

func TestTrackG722Clone(t *testing.T) {
	track := &TrackG722{}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackG722MediaDescription(t *testing.T) {
	track := &TrackG722{}

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"9"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "9 G722/8000",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
