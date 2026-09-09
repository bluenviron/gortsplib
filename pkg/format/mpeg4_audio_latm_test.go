package format_test //nolint:revive

import (
	"testing"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/mpeg4audio"
	"github.com/pion/rtp"
	"github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/format"
)

func TestMPEG4AudioLATMAttributes(t *testing.T) {
	format := &format.MPEG4AudioLATM{
		PayloadTyp:     96,
		ProfileLevelID: 1,
		StreamMuxConfig: &mpeg4audio.StreamMuxConfig{
			Programs: []*mpeg4audio.StreamMuxConfigProgram{{
				Layers: []*mpeg4audio.StreamMuxConfigLayer{{
					AudioSpecificConfig: &mpeg4audio.AudioSpecificConfig{
						Type:          2,
						SampleRate:    44100,
						ChannelConfig: 2,
						ChannelCount:  2, //nolint:staticcheck
					},
					LatmBufferFullness: 255,
				}},
			}},
		},
	}
	require.Equal(t, "MPEG-4 Audio LATM", format.Codec())
	require.Equal(t, 44100, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestMPEG4AudioLATMDecEncoder(t *testing.T) {
	format := &format.MPEG4AudioLATM{
		PayloadTyp:     96,
		ProfileLevelID: 1,
		StreamMuxConfig: &mpeg4audio.StreamMuxConfig{
			Programs: []*mpeg4audio.StreamMuxConfigProgram{{
				Layers: []*mpeg4audio.StreamMuxConfigLayer{{
					AudioSpecificConfig: &mpeg4audio.AudioSpecificConfig{
						Type:          2,
						SampleRate:    48000,
						ChannelConfig: 2,
						ChannelCount:  2, //nolint:staticcheck
					},
					LatmBufferFullness: 255,
				}},
			}},
		},
	}

	enc, err := format.CreateEncoder()
	require.NoError(t, err)

	pkts, err := enc.Encode([]byte{0x01, 0x02, 0x03, 0x04})
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec, err := format.CreateDecoder()
	require.NoError(t, err)

	byts, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, byts)
}

func TestMPEG4AudioLATMMultiLayerSameConfig(t *testing.T) {
	// A StreamMuxConfig where a non-first layer uses useSameConfig leaves that
	// layer's AudioSpecificConfig nil (a valid ISO-14496-3 encoding). Parsing
	// it must not panic. config below is a 2-layer config with the 2nd layer
	// reusing the first's AudioSpecificConfig.
	md := &sdp.MediaDescription{
		MediaName: sdp.MediaName{Media: "audio", Formats: []string{"96"}},
		Attributes: []sdp.Attribute{
			{Key: "rtpmap", Value: "96 MP4A-LATM/48000/2"},
			{Key: "fmtp", Value: "96 cpresent=0; object=2; config=400223203fe3fc"},
		},
	}
	f, err := format.Unmarshal(md, "96")
	require.NoError(t, err)
	require.NotNil(t, f)
}
