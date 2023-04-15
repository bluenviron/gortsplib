package formats

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
)

func TestMPEG4AudioGenericAttributes(t *testing.T) {
	format := &MPEG4AudioGeneric{
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
	require.Equal(t, "MPEG4-audio-gen", format.String())
	require.Equal(t, 48000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestMPEG4AudioGenericDecEncoder(t *testing.T) {
	format := &MPEG4AudioGeneric{
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
