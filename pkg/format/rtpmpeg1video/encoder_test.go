package rtpmpeg1video

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
	frame []byte
	pkts  []*rtp.Packet
}{
	{
		"single",
		bytes.Repeat([]byte{1, 2, 3, 4}, 240/4),
		[]*rtp.Packet{{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    32,
				SequenceNumber: 17645,
				SSRC:           0x9dbb7812,
			},
			Payload: mergeBytes(
				[]byte{0, 0, 0x18, 0},
				bytes.Repeat([]byte{1, 2, 3, 4}, 240/4),
			),
		}},
	},
	{
		"aggregated",
		mergeBytes(
			[]byte{0, 0, 1},
			bytes.Repeat([]byte{1, 2, 3, 4}, 128/4),
			[]byte{0, 0, 1},
			bytes.Repeat([]byte{5, 6, 7, 8}, 128/4),
		),
		[]*rtp.Packet{{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    32,
				SequenceNumber: 17645,
				SSRC:           0x9dbb7812,
			},
			Payload: mergeBytes(
				[]byte{0, 0, 0x18, 0},
				[]byte{0, 0, 1},
				bytes.Repeat([]byte{1, 2, 3, 4}, 128/4),
				[]byte{0, 0, 1},
				bytes.Repeat([]byte{5, 6, 7, 8}, 128/4),
			),
		}},
	},
	{
		"fragmented",
		mergeBytes(
			[]byte{0, 0, 1},
			bytes.Repeat([]byte{2}, 1500),
		),
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    32,
					SequenceNumber: 17645,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0, 0, 0x10, 0},
					[]byte{0, 0, 1},
					bytes.Repeat([]byte{2}, 993),
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    32,
					SequenceNumber: 17646,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0, 0, 0x08, 0},
					bytes.Repeat([]byte{2}, 507),
				),
			},
		},
	},
	{
		"fragmented to the limit",
		mergeBytes(
			[]byte{0, 0, 1},
			bytes.Repeat([]byte{2}, 1989),
		),
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    32,
					SequenceNumber: 17645,
					SSRC:           2646308882,
				},
				Payload: mergeBytes(
					[]byte{0, 0, 0x10, 0},
					[]byte{0, 0, 1},
					bytes.Repeat([]byte{2}, 993),
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    32,
					SequenceNumber: 17646,
					SSRC:           2646308882,
				},
				Payload: mergeBytes(
					[]byte{0, 0, 0x08, 0},
					bytes.Repeat([]byte{2}, 996),
				),
			},
		},
	},
}

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			e := &Encoder{
				SSRC:                  uint32Ptr(0x9dbb7812),
				InitialSequenceNumber: uint16Ptr(0x44ed),
				PayloadMaxSize:        1000,
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
	e := &Encoder{}
	err := e.Init()
	require.NoError(t, err)
	require.NotEqual(t, nil, e.SSRC)
	require.NotEqual(t, nil, e.InitialSequenceNumber)
}
