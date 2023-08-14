package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestH264Attributes(t *testing.T) {
	format := &H264{
		PayloadTyp:        96,
		SPS:               []byte{0x01, 0x02},
		PPS:               []byte{0x03, 0x04},
		PacketizationMode: 1,
	}
	require.Equal(t, "H264", format.Codec())
	require.Equal(t, 90000, format.ClockRate())

	sps, pps := format.SafeParams()
	require.Equal(t, []byte{0x01, 0x02}, sps)
	require.Equal(t, []byte{0x03, 0x04}, pps)

	format.SafeSetParams([]byte{0x07, 0x08}, []byte{0x09, 0x0A})

	sps, pps = format.SafeParams()
	require.Equal(t, []byte{0x07, 0x08}, sps)
	require.Equal(t, []byte{0x09, 0x0A}, pps)
}

func TestH264PTSEqualsDTS(t *testing.T) {
	format := &H264{
		PayloadTyp:        96,
		SPS:               []byte{0x01, 0x02},
		PPS:               []byte{0x03, 0x04},
		PacketizationMode: 1,
	}

	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{
		Payload: []byte{0x05},
	}))
	require.Equal(t, false, format.PTSEqualsDTS(&rtp.Packet{
		Payload: []byte{0x01},
	}))
}

func TestH264DecEncoder(t *testing.T) {
	format := &H264{}

	enc, err := format.CreateEncoder()
	require.NoError(t, err)

	pkts, err := enc.Encode([][]byte{{0x01, 0x02, 0x03, 0x04}})
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec, err := format.CreateDecoder()
	require.NoError(t, err)

	byts, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x01, 0x02, 0x03, 0x04}}, byts)
}
