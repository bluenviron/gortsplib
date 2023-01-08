package rtph265

import (
	"bytes"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

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
	pts   time.Duration
	pkts  []*rtp.Packet
}{
	{
		"single",
		[][]byte{{0x01, 0x02, 0x03, 0x04, 0x05}},
		25 * time.Millisecond,
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289528607,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			},
		},
	},
	{
		"aggregated",
		[][]byte{
			{0x07, 0x07},
			{0x08, 0x08},
			{0x09, 0x09},
		},
		0,
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
					0x60, 0x00, 0x00, 0x02, 0x07, 0x07, 0x00, 0x02,
					0x08, 0x08, 0x00, 0x02, 0x09, 0x09,
				},
			},
		},
	},
	{
		"fragmented",
		[][]byte{
			bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 1024),
		},
		55 * time.Millisecond,
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x63, 0x02, 0x80, 0x03, 0x04},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 363),
					[]byte{0x01, 0x02, 0x03},
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x63, 0x02, 0x00, 0x04},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 364),
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x63, 0x02, 0x40},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 295),
				),
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
			}
			e.Init()

			pkts, err := e.Encode(ca.nalus, ca.pts)
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
