package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackGenericNew(t *testing.T) {
	track, err := NewTrackGeneric(
		"video",
		[]string{"100", "101"},
		"98 H265/90000",
		"98 profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; "+
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	)
	require.NoError(t, err)
	require.Equal(t, "", track.GetControl())
	require.Equal(t, 90000, track.ClockRate())
}

func TestTrackGenericNewErrors(t *testing.T) {
	_, err := NewTrackGeneric(
		"video",
		[]string{"100", "101"},
		"98 H265/",
		"",
	)
	require.EqualError(t, err, "unable to get clock rate: strconv.ParseInt: parsing \"\": invalid syntax")
}

func TestTrackGenericClone(t *testing.T) {
	track, err := NewTrackGeneric(
		"video",
		[]string{"100", "101"},
		"98 H265/90000",
		"98 profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; "+
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	)
	require.NoError(t, err)

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackGenericMediaDescription(t *testing.T) {
	track, err := NewTrackGeneric(
		"video",
		[]string{"100", "101"},
		"98 H265/90000",
		"98 profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; "+
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	)
	require.NoError(t, err)
	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "video",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"100", "101"},
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
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
