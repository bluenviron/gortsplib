package rtpmpeg4video

import (
	"bytes"
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
	pkts  []*rtp.Packet
}{
	{
		"single",
		[]byte{0x01, 0x02, 0x03, 0x04},
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{
					0x01, 0x02, 0x03, 0x04,
				},
			},
		},
	},
	{
		"fragmented",
		bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 150/4),
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					SSRC:           0x9dbb7812,
				},
				Payload: bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 100/4),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					SSRC:           0x9dbb7812,
				},
				Payload: bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 50/4),
			},
		},
	},
}

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			e := &Encoder{
				PayloadType:           96,
				SSRC:                  uint32Ptr(0x9dbb7812),
				InitialSequenceNumber: uint16Ptr(0x44ed),
				PayloadMaxSize:        100,
			}
			err := e.Init()
			require.NoError(t, err)

			pkts, err := e.Encode(ca.frame)
			require.NoError(t, err)
			require.Equal(t, ca.pkts, pkts)
		})
	}
}

func TestEncodeRandomInitialState(t *testing.T) {
	e := &Encoder{
		PayloadType: 96,
	}
	err := e.Init()
	require.NoError(t, err)
	require.NotEqual(t, nil, e.SSRC)
	require.NotEqual(t, nil, e.InitialSequenceNumber)
}
