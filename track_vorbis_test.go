package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackVorbisAttributes(t *testing.T) {
	track := &TrackVorbis{
		PayloadType:   96,
		SampleRate:    48000,
		ChannelCount:  2,
		Configuration: []byte{0x01, 0x02, 0x03, 0x04},
	}
	require.Equal(t, "Vorbis", track.String())
	require.Equal(t, 48000, track.ClockRate())
	require.Equal(t, "", track.GetControl())
}

func TestTracVorbisClone(t *testing.T) {
	track := &TrackVorbis{
		PayloadType:   96,
		SampleRate:    48000,
		ChannelCount:  2,
		Configuration: []byte{0x01, 0x02, 0x03, 0x04},
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackVorbisMediaDescription(t *testing.T) {
	track := &TrackVorbis{
		PayloadType:   96,
		SampleRate:    48000,
		ChannelCount:  2,
		Configuration: []byte{0x01, 0x02, 0x03, 0x04},
	}

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "audio",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"96"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "96 VORBIS/48000/2",
			},
			{
				Key:   "fmtp",
				Value: "96 configuration=AQIDBA==",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
