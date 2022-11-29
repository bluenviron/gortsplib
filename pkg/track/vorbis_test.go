package track

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVorbisAttributes(t *testing.T) {
	track := &Vorbis{
		PayloadTyp:    96,
		SampleRate:    48000,
		ChannelCount:  2,
		Configuration: []byte{0x01, 0x02, 0x03, 0x04},
	}
	require.Equal(t, "Vorbis", track.String())
	require.Equal(t, 48000, track.ClockRate())
	require.Equal(t, uint8(96), track.PayloadType())
}

func TestTracVorbisClone(t *testing.T) {
	track := &Vorbis{
		PayloadTyp:    96,
		SampleRate:    48000,
		ChannelCount:  2,
		Configuration: []byte{0x01, 0x02, 0x03, 0x04},
	}

	clone := track.Clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestVorbisMediaDescription(t *testing.T) {
	track := &Vorbis{
		PayloadTyp:    96,
		SampleRate:    48000,
		ChannelCount:  2,
		Configuration: []byte{0x01, 0x02, 0x03, 0x04},
	}

	rtpmap, fmtp := track.Marshal()
	require.Equal(t, "VORBIS/48000/2", rtpmap)
	require.Equal(t, "configuration=AQIDBA==", fmtp)
}
