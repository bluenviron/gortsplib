package formats

import (
	"testing"

	"github.com/pion/rtp"
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
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))

	vps, sps, pps := format.SafeParams()
	require.Equal(t, []byte{0x01, 0x02}, vps)
	require.Equal(t, []byte{0x03, 0x04}, sps)
	require.Equal(t, []byte{0x05, 0x06}, pps)

	format.SafeSetParams([]byte{0x07, 0x08}, []byte{0x09, 0x0A}, []byte{0x0B, 0x0C})

	vps, sps, pps = format.SafeParams()
	require.Equal(t, []byte{0x07, 0x08}, vps)
	require.Equal(t, []byte{0x09, 0x0A}, sps)
	require.Equal(t, []byte{0x0B, 0x0C}, pps)
}

func TestH265DecEncoder(t *testing.T) {
	format := &H265{}

	enc := format.CreateEncoder()
	pkts, err := enc.Encode([][]byte{{0x01, 0x02, 0x03, 0x04}}, 0)
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec := format.CreateDecoder()
	byts, _, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x01, 0x02, 0x03, 0x04}}, byts)
}
