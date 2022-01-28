package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackGenericNewFromMediaDescription(t *testing.T) {
	for _, ca := range []struct {
		name  string
		md    *psdp.MediaDescription
		track *TrackGeneric
	}{
		{
			"pcma",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"8"},
				},
			},
			&TrackGeneric{
				clockRate: 8000,
				media:     "audio",
				formats:   []string{"8"},
			},
		},
		{
			"pcmu",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "audio",
					Port:    psdp.RangedPort{Value: 49170},
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"0"},
				},
			},
			&TrackGeneric{
				clockRate: 8000,
				media:     "audio",
				formats:   []string{"0"},
			},
		},
		{
			"multiple formats",
			&psdp.MediaDescription{
				MediaName: psdp.MediaName{
					Media:   "video",
					Port:    psdp.RangedPort{Value: 0},
					Protos:  []string{"RTP", "AVP"},
					Formats: []string{"98", "96"},
				},
				Attributes: []psdp.Attribute{
					{
						Key:   "rtpmap",
						Value: "98 H265/90000",
					},
					{
						Key: "fmtp",
						Value: "98 profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
							"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
					},
				},
			},
			&TrackGeneric{
				clockRate: 90000,
				media:     "video",
				formats:   []string{"98", "96"},
				rtpmap:    "98 H265/90000",
				fmtp: "98 profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
					"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			track, err := newTrackGenericFromMediaDescription(ca.md)
			require.NoError(t, err)
			require.Equal(t, ca.track, track)
		})
	}
}
