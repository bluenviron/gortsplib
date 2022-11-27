package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackGenericAttributes(t *testing.T) {
	track := &TrackGeneric{
		PayloadType: 98,
		RTPMap:      "H265/90000",
		FMTP: "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	}
	err := track.Init()
	require.NoError(t, err)

	require.Equal(t, "Generic", track.String())
	require.Equal(t, 90000, track.ClockRate())
}

func TestTrackGenericClone(t *testing.T) {
	track := &TrackGeneric{
		PayloadType: 98,
		RTPMap:      "H265/90000",
		FMTP: "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	}
	err := track.Init()
	require.NoError(t, err)

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackGenericMediaDescription(t *testing.T) {
	track := &TrackGeneric{
		PayloadType: 98,
		RTPMap:      "H265/90000",
		FMTP: "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	}
	err := track.Init()
	require.NoError(t, err)

	rtpmap, fmtp := track.marshal()
	require.Equal(t, "H265/90000", rtpmap)
	require.Equal(t, "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; "+
		"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=", fmtp)
}
