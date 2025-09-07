package rtph265

import (
	"bytes"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func ptrOf[T any](v T) *T {
	return &v
}

func mergeBytes(vals ...[]byte) []byte {
	size := 0
	for _, v := range vals {
		size += len(v)
	}
	res := make([]byte, size)

	pos := 0
	for _, v := range vals {
		n := copy(res[pos:], v)
		pos += n
	}

	return res
}

var cases = []struct {
	name  string
	nalus [][]byte
	pkts  []*rtp.Packet
}{
	{
		"single",
		[][]byte{{0x01, 0x02, 0x03, 0x04, 0x05}},
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			},
		},
	},
	{
		"aggregated",
		[][]byte{
			{0x07, 0x57},
			{0x08, 0x66},
			{0x09, 0x75},
		},
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
					0x60, 0x65, 0x00, 0x02, 0x07, 0x57, 0x00, 0x02,
					0x08, 0x66, 0x00, 0x02, 0x09, 0x75,
				},
			},
		},
	},
	{
		"fragmented",
		[][]byte{
			bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 375),
		},
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x63, 0x02, 0x80, 0x03, 0x04},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 248),
					[]byte{0x01, 0x02, 0x03},
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x63, 0x02, 0x40, 0x04},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 125),
				),
			},
		},
	},
	{
		"fragmented to the limit",
		[][]byte{bytes.Repeat([]byte{1}, 1996)},
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x63, 0x01, 0x80},
					bytes.Repeat([]byte{1}, 997),
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x63, 0x01, 0x40},
					bytes.Repeat([]byte{1}, 997),
				),
			},
		},
	},
}

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			e := &Encoder{
				PayloadType:           96,
				SSRC:                  ptrOf(uint32(0x9dbb7812)),
				InitialSequenceNumber: ptrOf(uint16(0x44ed)),
				PayloadMaxSize:        1000,
			}
			err := e.Init()
			require.NoError(t, err)

			pkts, err := e.Encode(ca.nalus)
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
