package formats

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestMPEG4AudioLATMAttributes(t *testing.T) {
	format := &MPEG4AudioLATM{
		PayloadTyp:     96,
		SampleRate:     48000,
		Channels:       2,
		Object:         2,
		ProfileLevelID: 1,
		Config:         []byte{0x01, 0x02, 0x03},
	}
	require.Equal(t, "MPEG4-audio-latm", format.String())
	require.Equal(t, 48000, format.ClockRate())
	require.Equal(t, uint8(96), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestMPEG4AudioLATMMediaDescription(t *testing.T) {
	format := &MPEG4AudioLATM{
		PayloadTyp:     96,
		SampleRate:     48000,
		Channels:       2,
		Object:         2,
		ProfileLevelID: 1,
		Config:         []byte{0x01, 0x02, 0x03},
	}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "MP4A-LATM/48000/2", rtpmap)
	require.Equal(t, map[string]string{
		"profile-level-id": "1",
		"object":           "2",
		"config":           "010203",
	}, fmtp)
}
