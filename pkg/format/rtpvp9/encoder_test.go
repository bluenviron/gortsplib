package rtpvp9

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
				Payload: []byte{0x9c, 0xb5, 0xaf, 0x01, 0x02, 0x03, 0x04},
			},
		},
	},
	{
		"fragmented",
		bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 4096/4),
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes([]byte{0x98, 0xb5, 0xaf}, bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 364), []byte{0x01}),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17646,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes([]byte{0x90, 0xb5, 0xaf, 0x02, 0x03, 0x04},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 363), []byte{0x01, 0x02}),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes([]byte{0x94, 0xb5, 0xaf, 0x03, 0x04}, bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 295)),
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
				InitialPictureID:      uint16Ptr(0x35af),
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
