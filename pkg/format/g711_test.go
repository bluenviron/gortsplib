package format

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestG711Attributes(t *testing.T) {
	format := &G711{}
	require.Equal(t, "G711", format.String())
	require.Equal(t, 8000, format.ClockRate())
	require.Equal(t, uint8(8), format.PayloadType())
	require.Equal(t, true, format.PTSEqualsDTS(&rtp.Packet{}))

	format = &G711{
		MULaw: true,
	}
	require.Equal(t, "G711", format.String())
	require.Equal(t, 8000, format.ClockRate())
	require.Equal(t, uint8(0), format.PayloadType())
}

func TestG711MediaDescription(t *testing.T) {
	t.Run("pcma", func(t *testing.T) {
		format := &G711{}

		rtpmap, fmtp := format.Marshal()
		require.Equal(t, "PCMA/8000", rtpmap)
		require.Equal(t, "", fmtp)
	})

	t.Run("pcmu", func(t *testing.T) {
		format := &G711{
			MULaw: true,
		}

		rtpmap, fmtp := format.Marshal()
		require.Equal(t, "PCMU/8000", rtpmap)
		require.Equal(t, "", fmtp)
	})
}

func TestG711DecEncoder(t *testing.T) {
	format := &G711{}

	enc := format.CreateEncoder()
	pkt, err := enc.Encode([]byte{0x01, 0x02, 0x03, 0x04}, 0)
	require.NoError(t, err)
	require.Equal(t, format.PayloadType(), pkt.PayloadType)

	dec := format.CreateDecoder()
	byts, _, err := dec.Decode(pkt)
	require.NoError(t, err)
	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, byts)
}
