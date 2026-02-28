package format //nolint:revive

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestMPEGTSAttributes(t *testing.T) {
	format := &MPEGTS{}
	require.Equal(t, "MPEG-TS", format.Codec())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestMPEGTSDecEncoder(t *testing.T) {
	format := &MPEGTS{}

	enc, err := format.CreateEncoder()
	require.NoError(t, err)

	// create a valid 188-byte TS packet
	tsData := make([]byte, 188)
	tsData[0] = 0x47

	pkts, err := enc.Encode([][]byte{tsData})
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec, err := format.CreateDecoder()
	require.NoError(t, err)

	byts, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, [][]byte{tsData}, byts)
}
