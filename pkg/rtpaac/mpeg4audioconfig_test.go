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
		"aac-lc 44.1khz mono",
		[]byte{0x12, 0x08, 0x56, 0xe5, 0x00},
		MPEG4AudioConfig{
			Type:         MPEG4AudioTypeAACLC,
			SampleRate:   44100,
			ChannelCount: 1,
		},
	},
	{
		"aac-lc 48khz stereo",
		[]byte{17, 144},
		MPEG4AudioConfig{
			Type:         MPEG4AudioTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
	},
	{
		"aac-lc 96khz stereo",
		[]byte{0x10, 0x10, 0x56, 0xE5, 0x00},
		MPEG4AudioConfig{
			Type:         MPEG4AudioTypeAACLC,
			SampleRate:   96000,
			ChannelCount: 2,
		},
	},
	{
		"aac-lc 44.1khz 5.1",
		[]byte{0x12, 0x30},
		MPEG4AudioConfig{
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

func TestConfigDecodeErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts []byte
		err  string
	}{
		{
			"empty",
			[]byte{},
			"EOF",
		},
		{
			"extended type missing",
			[]byte{31 << 3},
			"EOF",
		},
		{
			"extended type invalid",
			[]byte{31 << 3, 20},
			"unsupported type: 32",
		},
		{
			"sample rate missing",
			[]byte{0x12},
			"EOF",
		},
		{
			"sample rate invalid",
			[]byte{0x12 | 13>>5, 13 << 3},
			"invalid channel configuration: 13",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var dec MPEG4AudioConfig
			err := dec.Decode(ca.byts)
			require.Equal(t, ca.err, err.Error())
		})
	}
}
