package mpeg4audio

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var configCases = []struct {
	name string
	enc  []byte
	dec  Config
}{
	{
		"aac-lc 16khz mono",
		[]byte{0x14, 0x08},
		Config{
			Type:         ObjectTypeAACLC,
			SampleRate:   16000,
			ChannelCount: 1,
		},
	},
	{
		"aac-lc 44.1khz mono",
		[]byte{0x12, 0x08},
		Config{
			Type:         ObjectTypeAACLC,
			SampleRate:   44100,
			ChannelCount: 1,
		},
	},
	{
		"aac-lc 44.1khz 5.1",
		[]byte{0x12, 0x30},
		Config{
			Type:         ObjectTypeAACLC,
			SampleRate:   44100,
			ChannelCount: 6,
		},
	},
	{
		"aac-lc 48khz stereo",
		[]byte{17, 144},
		Config{
			Type:         ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
	},
	{
		"aac-lc 53khz stereo",
		[]byte{0x17, 0x80, 0x67, 0x84, 0x10},
		Config{
			Type:         ObjectTypeAACLC,
			SampleRate:   53000,
			ChannelCount: 2,
		},
	},
	{
		"aac-lc 96khz stereo delay",
		[]byte{0x10, 0x12, 0x0c, 0x08},
		Config{
			Type:               ObjectTypeAACLC,
			SampleRate:         96000,
			ChannelCount:       2,
			DependsOnCoreCoder: true,
			CoreCoderDelay:     385,
		},
	},
}

func TestConfigUnmarshal(t *testing.T) {
	for _, ca := range configCases {
		t.Run(ca.name, func(t *testing.T) {
			var dec Config
			err := dec.Unmarshal(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.dec, dec)
		})
	}
}

func TestConfigMarshal(t *testing.T) {
	for _, ca := range configCases {
		t.Run(ca.name, func(t *testing.T) {
			enc, err := ca.dec.Marshal()
			require.NoError(t, err)
			require.Equal(t, ca.enc, enc)
		})
	}
}

func TestConfigMarshalErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		conf Config
		err  string
	}{
		{
			"invalid channel config",
			Config{
				Type:         2,
				SampleRate:   44100,
				ChannelCount: 0,
			},
			"invalid channel count (0)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := ca.conf.Marshal()
			require.EqualError(t, err, ca.err)
		})
	}
}
