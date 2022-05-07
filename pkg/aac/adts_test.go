package aac

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var casesADTS = []struct {
	name string
	byts []byte
	pkts []*ADTSPacket
}{
	{
		"single",
		[]byte{0xff, 0xf1, 0x4c, 0x80, 0x1, 0x3f, 0xfc, 0xaa, 0xbb},
		[]*ADTSPacket{
			{
				Type:         2,
				SampleRate:   48000,
				ChannelCount: 2,
				AU:           []byte{0xaa, 0xbb},
			},
		},
	},
	{
		"multiple",
		[]byte{
			0xff, 0xf1, 0x50, 0x40, 0x1, 0x3f, 0xfc, 0xaa,
			0xbb, 0xff, 0xf1, 0x4c, 0x80, 0x1, 0x3f, 0xfc,
			0xcc, 0xdd,
		},
		[]*ADTSPacket{
			{
				Type:         2,
				SampleRate:   44100,
				ChannelCount: 1,
				AU:           []byte{0xaa, 0xbb},
			},
			{
				Type:         2,
				SampleRate:   48000,
				ChannelCount: 2,
				AU:           []byte{0xcc, 0xdd},
			},
		},
	},
}

func TestDecodeADTS(t *testing.T) {
	for _, ca := range casesADTS {
		t.Run(ca.name, func(t *testing.T) {
			pkts, err := DecodeADTS(ca.byts)
			require.NoError(t, err)
			require.Equal(t, ca.pkts, pkts)
		})
	}
}

func TestEncodeADTS(t *testing.T) {
	for _, ca := range casesADTS {
		t.Run(ca.name, func(t *testing.T) {
			byts, err := EncodeADTS(ca.pkts)
			require.NoError(t, err)
			require.Equal(t, ca.byts, byts)
		})
	}
}

func TestDecodeADTSErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		byts []byte
		err  string
	}{
		{
			"invalid length",
			[]byte{0x01},
			"invalid length",
		},
		{
			"invalid syncword",
			[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			"invalid syncword",
		},
		{
			"crc",
			[]byte{0xff, 0xF0, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
			"CRC is not supported",
		},
		{
			"invalid audio type",
			[]byte{0xff, 0xf1, 0x8c, 0x80, 0x1, 0x3f, 0xfc, 0xaa},
			"unsupported audio type: 3",
		},
		{
			"invalid sample rate index",
			[]byte{0xff, 0xf1, 0x74, 0x80, 0x1, 0x3f, 0xfc, 0xaa},
			"invalid sample rate index: 13",
		},
		{
			"invalid channel configuration",
			[]byte{0xff, 0xf1, 0x4c, 0x00, 0x1, 0x3f, 0xfc, 0xaa},
			"invalid channel configuration: 0",
		},
		{
			"multiple frame count",
			[]byte{0xff, 0xf1, 0x4c, 0x80, 0x1, 0x3f, 0xfd, 0xaa},
			"frame count greater than 1 is not supported",
		},
		{
			"invalid frame length",
			[]byte{0xff, 0xf1, 0x4c, 0x80, 0x1, 0x3f, 0xfc, 0xaa},
			"invalid frame length",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			_, err := DecodeADTS(ca.byts)
			require.EqualError(t, err, ca.err)
		})
	}
}
