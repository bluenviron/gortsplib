package format

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestH264Attributes(t *testing.T) {
	format := &H264{
		PayloadTyp:        96,
		SPS:               []byte{0x01, 0x02},
		PPS:               []byte{0x03, 0x04},
		PacketizationMode: 1,
	}
	require.Equal(t, "H264", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(96), format.PayloadType())
	require.Equal(t, []byte{0x01, 0x02}, format.SafeSPS())
	require.Equal(t, []byte{0x03, 0x04}, format.SafePPS())

	format.SafeSetSPS([]byte{0x07, 0x08})
	format.SafeSetPPS([]byte{0x09, 0x0A})
	require.Equal(t, []byte{0x07, 0x08}, format.SafeSPS())
	require.Equal(t, []byte{0x09, 0x0A}, format.SafePPS())
}

func TestH264Clone(t *testing.T) {
	format := &H264{
		PayloadTyp:        96,
		SPS:               []byte{0x01, 0x02},
		PPS:               []byte{0x03, 0x04},
		PacketizationMode: 1,
	}

	clone := format.Clone()
	require.NotSame(t, format, clone)
	require.Equal(t, format, clone)
}

func TestH264MediaDescription(t *testing.T) {
	t.Run("standard", func(t *testing.T) {
		format := &H264{
			PayloadTyp: 96,
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

		rtpmap, fmtp := format.Marshal()
		require.Equal(t, "H264/90000", rtpmap)
		require.Equal(t, "packetization-mode=1; "+
			"sprop-parameter-sets=Z2QADKw7ULBLQgAAAwACAAADAD0I,aO48gA==; profile-level-id=64000C", fmtp)
	})

	t.Run("no sps/pps", func(t *testing.T) {
		format := &H264{
			PayloadTyp:        96,
			PacketizationMode: 1,
		}

		rtpmap, fmtp := format.Marshal()
		require.Equal(t, "H264/90000", rtpmap)
		require.Equal(t, "packetization-mode=1", fmtp)
	})
}
