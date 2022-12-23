package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestG722Attributes(t *testing.T) {
	format := &G722{}
	require.Equal(t, "G722", format.String())
	require.Equal(t, 8000, format.ClockRate())
	require.Equal(t, uint8(9), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))
}

func TestG722MediaDescription(t *testing.T) {
	format := &G722{}

	rtpmap, fmtp := format.Marshal()
	require.Equal(t, "G722/8000", rtpmap)
	require.Equal(t, "", fmtp)
}

func TestG722DecEncoder(t *testing.T) {
	format := &G722{}

	enc := format.CreateEncoder()
	pkt, err := enc.Encode([]byte{0x01, 0x02, 0x03, 0x04}, 0)
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkt.PayloadType)

	dec := format.CreateDecoder()
	byts, _, err := dec.Decode(pkt)
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, byts)
}
