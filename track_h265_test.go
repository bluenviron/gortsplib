package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackH265Attributes(t *testing.T) {
	track := &TrackH265{
		PayloadType: 96,
		VPS:         []byte{0x01, 0x02},
		SPS:         []byte{0x03, 0x04},
		PPS:         []byte{0x05, 0x06},
	}
	require.Equal(t, 90000, track.ClockRate())
	require.Equal(t, "", track.GetControl())
	require.Equal(t, []byte{0x01, 0x02}, track.SafeVPS())
	require.Equal(t, []byte{0x03, 0x04}, track.SafeSPS())
	require.Equal(t, []byte{0x05, 0x06}, track.SafePPS())

	track.SafeSetVPS([]byte{0x07, 0x08})
	track.SafeSetSPS([]byte{0x09, 0x0A})
	track.SafeSetPPS([]byte{0x0B, 0x0C})
	require.Equal(t, []byte{0x07, 0x08}, track.SafeVPS())
	require.Equal(t, []byte{0x09, 0x0A}, track.SafeSPS())
	require.Equal(t, []byte{0x0B, 0x0C}, track.SafePPS())
}

func TestTrackH265Clone(t *testing.T) {
	track := &TrackH265{
		PayloadType: 96,
		VPS:         []byte{0x01, 0x02},
		SPS:         []byte{0x03, 0x04},
		PPS:         []byte{0x05, 0x06},
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackH265MediaDescription(t *testing.T) {
	track := &TrackH265{
		PayloadType: 96,
		VPS:         []byte{0x01, 0x02},
		SPS:         []byte{0x03, 0x04},
		PPS:         []byte{0x05, 0x06},
	}

	require.Equal(t, &psdp.MediaDescription{
		MediaName: psdp.MediaName{
			Media:   "video",
			Protos:  []string{"RTP", "AVP"},
			Formats: []string{"96"},
		},
		Attributes: []psdp.Attribute{
			{
				Key:   "rtpmap",
				Value: "96 H265/90000",
			},
			{
				Key:   "fmtp",
				Value: "96 sprop-vps=AQI=; sprop-sps=AwQ=; sprop-pps=BQY=",
			},
			{
				Key:   "control",
				Value: "",
			},
		},
	}, track.MediaDescription())
}
