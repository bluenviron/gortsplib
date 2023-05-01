package rtpmpeg4video

import (
	"bytes"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

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
					Timestamp:      2289526357,
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
					Timestamp:      2289526357,
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
					Timestamp:      2289526357,
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
				PayloadType: 96,
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
				PayloadMaxSize: 100,
			}
			e.Init()

			pkts, err := e.Encode(ca.frame, 0)
			require.NoError(t, err)
			require.Equal(t, ca.pkts, pkts)
		})
	}
}

func TestEncodeRandomInitialState(t *testing.T) {
	e := &Encoder{
		PayloadType: 96,
	}
	e.Init()
	require.NotEqual(t, nil, e.SSRC)
	require.NotEqual(t, nil, e.InitialSequenceNumber)
	require.NotEqual(t, nil, e.InitialTimestamp)
}
