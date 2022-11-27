package gortsplib

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackH264Attributes(t *testing.T) {
	track := &TrackH264{
		PayloadType:       96,
		SPS:               []byte{0x01, 0x02},
		PPS:               []byte{0x03, 0x04},
		PacketizationMode: 1,
	}
	require.Equal(t, "H264", track.String())
	require.Equal(t, 90000, track.ClockRate())
	require.Equal(t, []byte{0x01, 0x02}, track.SafeSPS())
	require.Equal(t, []byte{0x03, 0x04}, track.SafePPS())

	track.SafeSetSPS([]byte{0x07, 0x08})
	track.SafeSetPPS([]byte{0x09, 0x0A})
	require.Equal(t, []byte{0x07, 0x08}, track.SafeSPS())
	require.Equal(t, []byte{0x09, 0x0A}, track.SafePPS())
}

func TestTrackH264Clone(t *testing.T) {
	track := &TrackH264{
		PayloadType:       96,
		SPS:               []byte{0x01, 0x02},
		PPS:               []byte{0x03, 0x04},
		PacketizationMode: 1,
	}

	clone := track.clone()
	require.NotSame(t, track, clone)
	require.Equal(t, track, clone)
}

func TestTrackH264MediaDescription(t *testing.T) {
	t.Run("standard", func(t *testing.T) {
		track := &TrackH264{
			PayloadType: 96,
			SPS: []byte{
				0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
				0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
				0x00, 0x03, 0x00, 0x3d, 0x08,
			},
			PPS: []byte{
				0x68, 0xee, 0x3c, 0x80,
			},
			PacketizationMode: 1,
		}

		rtpmap, fmtp := track.marshal()
		require.Equal(t, "H264/90000", rtpmap)
		require.Equal(t, "packetization-mode=1; "+
			"sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==; profile-level-id=64000C", fmtp)
	})

	t.Run("no sps/pps", func(t *testing.T) {
		track := &TrackH264{
			PayloadType:       96,
			PacketizationMode: 1,
		}

		rtpmap, fmtp := track.marshal()
		require.Equal(t, "H264/90000", rtpmap)
		require.Equal(t, "packetization-mode=1", fmtp)
	})
}
