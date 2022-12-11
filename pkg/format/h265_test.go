package format

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestH265Attributes(t *testing.T) {
	format := &H265{
		PayloadTyp: 96,
		VPS:        []byte{0x01, 0x02},
		SPS:        []byte{0x03, 0x04},
		PPS:        []byte{0x05, 0x06},
	}
	require.Equal(t, "H265", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, uint8(96), format.PayloadType())
	require.Equal(t, []byte{0x01, 0x02}, format.SafeVPS())
	require.Equal(t, []byte{0x03, 0x04}, format.SafeSPS())
	require.Equal(t, []byte{0x05, 0x06}, format.SafePPS())

	format.SafeSetVPS([]byte{0x07, 0x08})
	format.SafeSetSPS([]byte{0x09, 0x0A})
	format.SafeSetPPS([]byte{0x0B, 0x0C})
	require.Equal(t, []byte{0x07, 0x08}, format.SafeVPS())
	require.Equal(t, []byte{0x09, 0x0A}, format.SafeSPS())
	require.Equal(t, []byte{0x0B, 0x0C}, format.SafePPS())
}

func TestH265Clone(t *testing.T) {
	format := &H265{
		PayloadTyp: 96,
		VPS:        []byte{0x01, 0x02},
		SPS:        []byte{0x03, 0x04},
		PPS:        []byte{0x05, 0x06},
	}

	clone := format.Clone()
	require.NotSame(t, format, clone)
	require.Equal(t, format, clone)
}

func TestH265MediaDescription(t *testing.T) {
	format := &H265{
		PayloadTyp: 96,
		VPS:        []byte{0x01, 0x02},
		SPS:        []byte{0x03, 0x04},
		PPS:        []byte{0x05, 0x06},
	}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "H265/90000", rtpmap)
	require.Equal(t, "sprop-vps=AQI=; sprop-sps=AwQ=; sprop-pps=BQY=", fmtp)
}
