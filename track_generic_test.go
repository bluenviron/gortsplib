package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackGenericClone(t *testing.T) {
	track, err := newTrackGenericFromMediaDescription(
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
		})
	require.NoError(t, err)

	copy := track.clone()
	require.NotSame(t, track, copy)
	require.Equal(t, track, copy)
}
