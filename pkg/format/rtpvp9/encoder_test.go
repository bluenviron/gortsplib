package rtpvp9

import (
	"bytes"
	"testing"

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
	frame []byte
	pkts  []*rtp.Packet
}{
	{
		"single",
		[]byte{0x82, 0x49, 0x83, 0x42, 0x0, 0x77, 0xf0, 0x32, 0x34},
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
					0x8f, 0xb5, 0xaf, 0x18, 0x07, 0x80, 0x03, 0x24,
					0x01, 0x14, 0x01, 0x82, 0x49, 0x83, 0x42, 0x00,
					0x77, 0xf0, 0x32, 0x34,
				},
			},
		},
	},
	{
		"fragmented",
		mergeBytes(
			[]byte{0x82, 0x49, 0x83, 0x42, 0x0, 0x77, 0xf0, 0x32, 0x34},
			bytes.Repeat([]byte{1, 2, 3, 4}, 350),
		),
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
					[]byte{
						0x8b, 0xb5, 0xaf, 0x18, 0x07, 0x80, 0x03, 0x24,
						0x01, 0x14, 0x01, 0x82, 0x49, 0x83, 0x42, 0x00,
						0x77, 0xf0, 0x32, 0x34,
					},
					bytes.Repeat([]byte{1, 2, 3, 4}, 120)),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17646,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x81, 0xb5, 0xaf},
					bytes.Repeat([]byte{1, 2, 3, 4}, 124),
					[]byte{1},
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x85, 0xb5, 0xaf},
					[]byte{2, 3, 4},
					bytes.Repeat([]byte{1, 2, 3, 4}, 105)),
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
				InitialPictureID:      ptrOf(uint16(0x35af)),
				PayloadMaxSize:        500,
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
