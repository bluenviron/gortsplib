package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestVorbisAttributes(t *testing.T) {
	format := &Vorbis{
		PayloadTyp:    96,
		SampleRate:    48000,
		ChannelCount:  2,
		Configuration: []byte{0x01, 0x02, 0x03, 0x04},
	}
	require.Equal(t, "Vorbis", format.Codec())
	require.Equal(t, 48000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func FuzzUnmarshalVorbis(f *testing.F) {
	f.Fuzz(func(_ *testing.T, a, b string) {
		fo, err := Unmarshal("audio", 96, "Vorbis/"+a, map[string]string{
			"configuration": b,
		})
		if err == nil {
			fo.RTPMap()
			fo.FMTP()
		}
	})
}
