package formats

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestMPEG4VideoESAttributes(t *testing.T) {
	format := &MPEG4VideoES{
		PayloadTyp:     96,
		ProfileLevelID: 1,
		Config:         []byte{0x01, 0x02, 0x03},
	}
	require.Equal(t, "MPEG4-video-es", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(96), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestMPEG4VideoESMediaDescription(t *testing.T) {
	format := &MPEG4VideoES{
		PayloadTyp:     96,
		ProfileLevelID: 1,
		Config:         []byte{0x0a, 0x0b, 0x03},
	}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "MP4V-ES/90000", rtpmap)
	require.Equal(t, map[string]string{
		"profile-level-id": "1",
		"config":           "0A0B03",
	}, fmtp)
}
