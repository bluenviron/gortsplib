package rtpsimpleaudio

import (
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

var cases = []struct {
	name  string
	frame []byte
	pts   time.Duration
	pkt   *rtp.Packet
}{
	{
		"single",
		[]byte{0x01, 0x02, 0x03, 0x04},
		25 * time.Millisecond,
		&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    0,
				SequenceNumber: 17645,
				Timestamp:      2289526557,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x01, 0x02, 0x03, 0x04},
		},
	},
}

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			e := &Encoder{
				PayloadType: 0,
				SampleRate:  8000,
				SSRC: func() *uint32 {
					v := uint32(0x9dbb7812)
					return &v
				}(),
				InitialSequenceNumber: func() *uint16 {
					v := uint16(0x44ed)
					return &v
				}(),
				InitialTimestamp: func() *uint32 {
					v := uint32(0x88776655)
					return &v
				}(),
			}
			e.Init()

			pkt, err := e.Encode(ca.frame, ca.pts)
			require.NoError(t, err)
			require.Equal(t, ca.pkt, pkt)
		})
	}
}

func TestEncodeRandomInitialState(t *testing.T) {
	e := &Encoder{
		PayloadType: 0,
		SampleRate:  8000,
	}
	e.Init()
	require.NotEqual(t, nil, e.SSRC)
	require.NotEqual(t, nil, e.InitialSequenceNumber)
	require.NotEqual(t, nil, e.InitialTimestamp)
}
