package rtpsimpleaudio

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func uint16Ptr(v uint16) *uint16 {
	return &v
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}

var cases = []struct {
	name  string
	frame []byte
	pkt   *rtp.Packet
}{
	{
		"single",
		[]byte{0x01, 0x02, 0x03, 0x04},
		&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    0,
				SequenceNumber: 17645,
				Timestamp:      2289526357,
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
				PayloadType:           0,
				SampleRate:            8000,
				SSRC:                  uint32Ptr(0x9dbb7812),
				InitialSequenceNumber: uint16Ptr(0x44ed),
				InitialTimestamp:      uint32Ptr(0x88776655),
			}
			e.Init()

			pkt, err := e.Encode(ca.frame, 0)
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
