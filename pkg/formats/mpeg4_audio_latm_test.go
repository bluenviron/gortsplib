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
	require.Equal(t, "MPEG4-audio-latm", format.String())
	require.Equal(t, 44100, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}
