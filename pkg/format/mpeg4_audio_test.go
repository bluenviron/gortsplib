package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
)

func TestMPEG4AudioAttributes(t *testing.T) {
	t.Run("generic", func(t *testing.T) {
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
		require.Equal(t, "MPEG-4 Audio", format.Codec())
		require.Equal(t, 48000, format.ClockRate())
		require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
		require.Equal(t, &mpeg4audio.Config{
			Type:         mpeg4audio.ObjectTypeAACLC,
			SampleRate:   48000,
			ChannelCount: 2,
		}, format.GetConfig())
	})

	t.Run("latm", func(t *testing.T) {
		format := &MPEG4Audio{
			LATM:           true,
			PayloadTyp:     96,
			ProfileLevelID: 1,
			StreamMuxConfig: &mpeg4audio.StreamMuxConfig{
				Programs: []*mpeg4audio.StreamMuxConfigProgram{{
					Layers: []*mpeg4audio.StreamMuxConfigLayer{{
						AudioSpecificConfig: &mpeg4audio.Config{
							Type:         2,
							SampleRate:   44100,
							ChannelCount: 2,
						},
						LatmBufferFullness: 255,
					}},
				}},
			},
		}
		require.Equal(t, "MPEG-4 Audio", format.Codec())
		require.Equal(t, 44100, format.ClockRate())
		require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
		require.Equal(t, &mpeg4audio.Config{
			Type:         2,
			SampleRate:   44100,
			ChannelCount: 2,
		}, format.GetConfig())
	})
}

func TestMPEG4AudioDecEncoder(t *testing.T) {
	t.Run("generic", func(t *testing.T) {
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

		enc, err := format.CreateEncoder()
		require.NoError(t, err)

		pkts, err := enc.Encode([][]byte{{0x01, 0x02, 0x03, 0x04}})
		require.NoError(t, err)
		require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

		dec, err := format.CreateDecoder()
		require.NoError(t, err)

		byts, err := dec.Decode(pkts[0])
		require.NoError(t, err)
		require.Equal(t, [][]byte{{0x01, 0x02, 0x03, 0x04}}, byts)
	})

	t.Run("latm", func(t *testing.T) {
		format := &MPEG4Audio{
			LATM:           true,
			PayloadTyp:     96,
			ProfileLevelID: 1,
			StreamMuxConfig: &mpeg4audio.StreamMuxConfig{
				Programs: []*mpeg4audio.StreamMuxConfigProgram{{
					Layers: []*mpeg4audio.StreamMuxConfigLayer{{
						AudioSpecificConfig: &mpeg4audio.Config{
							Type:         2,
							SampleRate:   48000,
							ChannelCount: 2,
						},
						LatmBufferFullness: 255,
					}},
				}},
			},
		}

		enc, err := format.CreateEncoder()
		require.NoError(t, err)

		pkts, err := enc.Encode([][]byte{{0x01, 0x02, 0x03, 0x04}})
		require.NoError(t, err)
		require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

		dec, err := format.CreateDecoder()
		require.NoError(t, err)

		byts, err := dec.Decode(pkts[0])
		require.NoError(t, err)
		require.Equal(t, [][]byte{{0x01, 0x02, 0x03, 0x04}}, byts)
	})
}
