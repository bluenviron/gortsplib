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
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
