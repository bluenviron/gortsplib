package rtpmpegts

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			e := &Encoder{
				SSRC:                  ptrOf(uint32(0x12345678)),
				InitialSequenceNumber: ptrOf(uint16(1000)),
				PayloadMaxSize:        800,
			}
			err := e.Init()
			require.NoError(t, err)

			pkts, err := e.Encode(ca.ts)
			require.NoError(t, err)
			require.Equal(t, ca.rtp, pkts)
		})
	}
}
