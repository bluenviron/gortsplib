package rtplpcm

import (
	"bytes"
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

var cases = []struct {
	name    string
	samples []byte
	pts     time.Duration
	pkts    []*rtp.Packet
}{
	{
		"single",
		[]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
		25 * time.Millisecond,
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289527557,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			},
		},
	},
	{
		"splitted",
		bytes.Repeat([]byte{0x41, 0x42, 0x43}, 680),
		25 * time.Millisecond,
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289527557,
					SSRC:           0x9dbb7812,
				},
				Payload: bytes.Repeat([]byte{0x41, 0x42, 0x43}, 486),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289527800,
					SSRC:           0x9dbb7812,
				},
				Payload: bytes.Repeat([]byte{0x41, 0x42, 0x43}, 194),
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
				BitDepth:     24,
				SampleRate:   48000,
				ChannelCount: 2,
			}
			e.Init()

			pkts, err := e.Encode(ca.samples, ca.pts)
			require.NoError(t, err)
			require.Equal(t, ca.pkts, pkts)
		})
	}
}

func TestEncodeRandomInitialState(t *testing.T) {
	e := &Encoder{
		PayloadType:  96,
		BitDepth:     24,
		SampleRate:   48000,
		ChannelCount: 2,
	}
	e.Init()
	require.NotEqual(t, nil, e.SSRC)
	require.NotEqual(t, nil, e.InitialSequenceNumber)
	require.NotEqual(t, nil, e.InitialTimestamp)
}
