package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/v2/pkg/codecs/mpeg4audio"
)

func TestMPEG4AudioAttributes(t *testing.T) {
	format := &MPEG4Audio{
		PayloadTyp: 96,
		Config: &mpeg4audio.Config{
			Type:         mpeg4audio.ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}
	require.Equal(t, "MPEG4-audio", format.String())
	require.Equal(t, 48000, format.ClockRate())
	require.Equal(t, uint8(96), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestMPEG4AudioMediaDescription(t *testing.T) {
	format := &MPEG4Audio{
		PayloadTyp: 96,
		Config: &mpeg4audio.Config{
			Type:         mpeg4audio.ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "mpeg4-generic/48000/2", rtpmap)
	require.Equal(t, "profile-level-id=1; mode=AAC-hbr; sizelength=13;"+
		" indexlength=3; indexdeltalength=3; config=1190", fmtp)
}

func TestMPEG4AudioDecEncoder(t *testing.T) {
	format := &MPEG4Audio{
		PayloadTyp: 96,
		Config: &mpeg4audio.Config{
			Type:         mpeg4audio.ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		},
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}

	enc := format.CreateEncoder()
	pkts, err := enc.Encode([][]byte{{0x01, 0x02, 0x03, 0x04}}, 0)
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec := format.CreateDecoder()
	byts, _, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x01, 0x02, 0x03, 0x04}}, byts)
}
