package rtpmpeg4audio

import (
	"bytes"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

var casesLATM = []struct {
	name string
	au   []byte
	pkts []*rtp.Packet
}{
	{
		"single",
		[]byte{1, 2, 3, 4},
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					SSRC:           2646308882,
				},
				Payload: []byte{
					0x04, 0x01, 0x02, 0x03, 0x04,
				},
			},
		},
	},
	{
		"fragmented",
		bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7}, 512),
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					SSRC:           2646308882,
				},
				Payload: mergeBytes(
					bytes.Repeat([]byte{0xff}, 16),
					[]byte{0x10},
					bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7}, 180),
					[]byte{0, 1, 2},
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17646,
					SSRC:           2646308882,
				},
				Payload: mergeBytes(
					[]byte{3, 4, 5, 6, 7},
					bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7}, 181),
					[]byte{0, 1, 2, 3, 4, 5, 6},
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					SSRC:           2646308882,
				},
				Payload: mergeBytes(
					[]byte{7},
					bytes.Repeat([]byte{0, 1, 2, 3, 4, 5, 6, 7}, 149),
				),
			},
		},
	},
	{
		"fragmented to the limit",
		bytes.Repeat([]byte{1}, 2908),
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					SSRC:           2646308882,
				},
				Payload: mergeBytes(
					bytes.Repeat([]byte{0xff}, 11),
					[]byte{0x67},
					bytes.Repeat([]byte{1}, 1448),
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					SSRC:           2646308882,
				},
				Payload: mergeBytes(
					bytes.Repeat([]byte{1}, 1460),
				),
			},
		},
	},
}

func TestEncodeLATM(t *testing.T) {
	for _, ca := range casesLATM {
		t.Run(ca.name, func(t *testing.T) {
			e := &Encoder{
				LATM:                  true,
				PayloadType:           96,
				SSRC:                  uint32Ptr(0x9dbb7812),
				InitialSequenceNumber: uint16Ptr(0x44ed),
			}
			err := e.Init()
			require.NoError(t, err)

			pkts, err := e.Encode([][]byte{ca.au})
			require.NoError(t, err)
			require.Equal(t, ca.pkts, pkts)
		})
	}
}
