package gortsplib

import (
	"testing"

	psdp "github.com/pion/sdp/v3"
	"github.com/stretchr/testify/require"
)

func TestTrackH265New(t *testing.T) {
	track := NewTrackH265(96,
		[]byte{0x01, 0x02}, []byte{0x03, 0x04}, []byte{0x05, 0x06})
	require.Equal(t, "", track.GetControl())
	require.Equal(t, []byte{0x01, 0x02}, track.VPS())
	require.Equal(t, []byte{0x03, 0x04}, track.SPS())
	require.Equal(t, []byte{0x05, 0x06}, track.PPS())
}

func TestTrackH265Clone(t *testing.T) {
	track := NewTrackH265(96, []byte{0x01, 0x02}, []byte{0x03, 0x04}, []byte{0x05, 0x06})

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackH265MediaDescription(t *testing.T) {
	track := NewTrackH265(96, []byte{0x01, 0x02}, []byte{0x03, 0x04}, []byte{0x05, 0x06})

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
