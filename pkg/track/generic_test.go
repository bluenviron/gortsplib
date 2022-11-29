package track

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenericAttributes(t *testing.T) {
	track := &Generic{
		PayloadTyp: 98,
		RTPMap:     "H265/90000",
		FMTP: "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	}
	err := track.Init()
	require.NoError(t, err)

	require.Equal(t, "Generic", track.String())
	require.Equal(t, 90000, track.ClockRate())
	require.Equal(t, uint8(98), track.PayloadType())
}

func TestGenericClone(t *testing.T) {
	track := &Generic{
		PayloadTyp: 98,
		RTPMap:     "H265/90000",
		FMTP: "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	}
	err := track.Init()
	require.NoError(t, err)

	clone := track.Clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestGenericMediaDescription(t *testing.T) {
	track := &Generic{
		PayloadTyp: 98,
		RTPMap:     "H265/90000",
		FMTP: "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	}
	err := track.Init()
	require.NoError(t, err)

	rtpmap, fmtp := track.Marshal()
	require.Equal(t, "H265/90000", rtpmap)
	require.Equal(t, "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; "+
		"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=", fmtp)
}
