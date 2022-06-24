package aac

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
		"aac-lc 16khz mono",
		[]byte{0x14, 0x08},
		MPEG4AudioConfig{
			Type:         MPEG4AudioTypeAACLC,
			SampleRate:   16000,
			ChannelCount: 1,
		},
	},
	{
		"aac-lc 44.1khz mono",
		[]byte{0x12, 0x08, 0x56, 0xe5, 0x00},
		MPEG4AudioConfig{
			Type:              MPEG4AudioTypeAACLC,
			SampleRate:        44100,
			ChannelCount:      1,
			AOTSpecificConfig: []byte{0x0A, 0xDC, 0xA0},
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
		"aac-lc 53khz stereo",
		[]byte{0x17, 0x80, 0x67, 0x84, 0x10},
		MPEG4AudioConfig{
			Type:         MPEG4AudioTypeAACLC,
			SampleRate:   53000,
			ChannelCount: 2,
		},
	},
	{
		"aac-lc 96khz stereo",
		[]byte{0x10, 0x10, 0x56, 0xE5, 0x00},
		MPEG4AudioConfig{
			Type:              MPEG4AudioTypeAACLC,
			SampleRate:        96000,
			ChannelCount:      2,
			AOTSpecificConfig: []byte{0x0A, 0xDC, 0xA0},
		},
	},
}

func TestConfigUnmarshal(t *testing.T) {
	for _, ca := range configCases {
		t.Run(ca.name, func(t *testing.T) {
			var dec MPEG4AudioConfig
			err := dec.Unmarshal(ca.enc)
			require.NoError(t, err)
			require.Equal(t, ca.dec, dec)
		})
	}
}

func TestConfigUnmarshalErrors(t *testing.T) {
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
			"unsupported type",
			[]byte{18 << 3},
			"unsupported type: 18",
		},
		{
			"sample rate missing",
			[]byte{0x12},
			"EOF",
		},
		{
			"sample rate invalid",
			[]byte{0x17, 0},
			"invalid sample rate index (14)",
		},
		{
			"explicit sample rate missing",
			[]byte{0x17, 0x80, 0x67},
			"EOF",
		},
		{
			"channel configuration invalid",
			[]byte{0x11, 0xF0},
			"invalid channel configuration (14)",
		},
		{
			"channel configuration zero",
			[]byte{0x11, 0x80},
			"not yet supported",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var dec MPEG4AudioConfig
			err := dec.Unmarshal(ca.byts)
			require.EqualError(t, err, ca.err)
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
		conf MPEG4AudioConfig
		err  string
	}{
		{
			"invalid channel config",
			MPEG4AudioConfig{
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
