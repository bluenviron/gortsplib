package formats //nolint:dupl

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestAV1Attributes(t *testing.T) {
	format := &AV1{
		PayloadTyp: 100,
	}
	require.Equal(t, "AV1", format.String())
	require.Equal(t, 90000, format.ClockRate())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestAV1DecEncoder(t *testing.T) {
	format := &AV1{}

	enc := format.CreateEncoder()
	pkts, err := enc.Encode([][]byte{{0x01, 0x02, 0x03, 0x04}}, 0)
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkts[0].PayloadType)

	dec := format.CreateDecoder()
	byts, _, err := dec.Decode(pkts[0])
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x01, 0x02, 0x03, 0x04}}, byts)
}
