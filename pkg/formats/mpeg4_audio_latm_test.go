package formats

import (
	"testing"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestMPEG4AudioLATMAttributes(t *testing.T) {
	format := &MPEG4AudioLATM{
		PayloadTyp:     96,
		ProfileLevelID: 1,
		Config: &mpeg4audio.StreamMuxConfig{
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
	require.Equal(t, "MPEG4-audio", format.String())
	require.Equal(t, 44100, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestMPEG4AudioLATMDecEncoder(t *testing.T) {
	format := &MPEG4AudioLATM{
		PayloadTyp:     96,
		ProfileLevelID: 1,
		Config: &mpeg4audio.StreamMuxConfig{
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

	enc, err := format.CreateEncoder2()
	require.NoError(t, err)

	pkts, err := enc.Encode([]byte{0x01, 0x02, 0x03, 0x04}, 0)
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec, err := format.CreateDecoder2()
	require.NoError(t, err)

	byts, _, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, byts)
}
