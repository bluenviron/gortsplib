package rtpaac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var configCases = []struct {
	name string
	enc  []byte
	dec  MPEG4AudioConfig
}{
	{
		name: "aac-lc 48khz stereo",
		enc:  []byte{17, 144},
		dec: MPEG4AudioConfig{
			Type:         MPEG4AudioTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
	},
	{
		name: "aac-lc 96khz stereo",
		enc:  []byte{0x10, 0x10, 0x56, 0xE5, 0x00},
		dec: MPEG4AudioConfig{
			Type:         MPEG4AudioTypeAACLC,
			SampleRate:   96000,
			ChannelCount: 2,
		},
	},
	{
		name: "aac-lc 44.1khz 5.1",
		enc:  []byte{0x12, 0x30},
		dec: MPEG4AudioConfig{
			Type:         MPEG4AudioTypeAACLC,
			SampleRate:   44100,
			ChannelCount: 6,
		},
	},
}

func TestConfigDecode(t *testing.T) {
	for _, ca := range configCases {
		t.Run(ca.name, func(t *testing.T) {
			var dec MPEG4AudioConfig
			err := dec.Decode(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.dec, dec)
		})
	}
}
