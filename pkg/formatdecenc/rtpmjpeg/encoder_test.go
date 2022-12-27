package rtpmjpeg

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			e := &Encoder{
				SSRC: func() *uint32 {
					v := uint32(0x9dbb7812)
					return &v
				}(),
				InitialSequenceNumber: func() *uint16 {
					v := uint16(0x44ed)
					return &v
				}(),
				InitialTimestamp: func() *uint32 {
					v := uint32(2289528607)
					return &v
				}(),
			}
			e.Init()

			pkts, err := e.Encode(ca.image, ca.pts)
			require.NoError(t, err)
			require.Equal(t, ca.pkts, pkts)
		})
	}
}
