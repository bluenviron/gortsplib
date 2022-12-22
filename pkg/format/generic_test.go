package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestGenericAttributes(t *testing.T) {
	format := &Generic{
		PayloadTyp: 98,
		RTPMap:     "H265/90000",
		FMTP: "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	}
	err := format.Init()
	require.NoError(t, err)

	require.Equal(t, "Generic", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(98), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestGenericMediaDescription(t *testing.T) {
	format := &Generic{
		PayloadTyp: 98,
		RTPMap:     "H265/90000",
		FMTP: "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; " +
			"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=",
	}
	err := format.Init()
	require.NoError(t, err)

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "H265/90000", rtpmap)
	require.Equal(t, "profile-id=1; sprop-vps=QAEMAf//AWAAAAMAAAMAAAMAAAMAlqwJ; "+
		"sprop-sps=QgEBAWAAAAMAAAMAAAMAAAMAlqADwIAQ5Za5JMmuWcBSSgAAB9AAAHUwgkA=; sprop-pps=RAHgdrAwxmQ=", fmtp)
}
