package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackG711Attributes(t *testing.T) {
	track := &TrackG711{}
	require.Equal(t, "G711", track.String())
	require.Equal(t, 8000, track.ClockRate())
	require.Equal(t, "", track.GetControl())
}

func TestTrackG711Clone(t *testing.T) {
	track := &TrackG711{}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackG711MediaDescription(t *testing.T) {
	t.Run("pcma", func(t *testing.T) {
		track := &TrackG711{}

		require.Equal(t, &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   "audio",
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{"8"},
			},
			Attributes: []psdp.Attribute{
				{
					Key:   "rtpmap",
					Value: "8 PCMA/8000",
				},
				{
					Key:   "control",
					Value: "",
				},
			},
		}, track.MediaDescription())
	})

	t.Run("pcmu", func(t *testing.T) {
		track := &TrackG711{
			MULaw: true,
		}

		require.Equal(t, &psdp.MediaDescription{
			MediaName: psdp.MediaName{
				Media:   "audio",
				Protos:  []string{"RTP", "AVP"},
				Formats: []string{"0"},
			},
			Attributes: []psdp.Attribute{
				{
					Key:   "rtpmap",
					Value: "0 PCMU/8000",
				},
				{
					Key:   "control",
					Value: "",
				},
			},
		}, track.MediaDescription())
	})
}
