package rtpmpegts_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/format/rtpmpegts"
)

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			e := &rtpmpegts.Encoder{
				SSRC:                  new(uint32(0x12345678)),
				InitialSequenceNumber: new(uint16(1000)),
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
