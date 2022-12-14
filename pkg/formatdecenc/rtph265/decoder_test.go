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

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			d.Init()

			// send an initial packet downstream
			// in order to compute the right timestamp,
			// that is relative to the initial packet
			pkt := rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x06, 0x00},
			}
			_, _, err := d.Decode(&pkt)
			require.NoError(t, err)

			var nalus [][]byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				addNALUs, pts, err := d.Decode(pkt)
				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
				require.Equal(t, ca.pts, pts)
				nalus = append(nalus, addNALUs...)

				// test input integrity
				require.Equal(t, clone, pkt)
			}

			require.Equal(t, ca.nalus, nalus)
		})
	}
}

func TestDecodeErrors(t *testing.T) {
	for _, ca := range []struct {
		name string
		pkts []*rtp.Packet
		err  string
	}{
		{
			"missing payload",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
				},
			},
			"payload is too short",
		},
		{
			"aggregation unit no size",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{48 << 1, 0x00, 0x01},
				},
			},
			"invalid aggregation unit (invalid size)",
		},
		{
			"aggregation unit invalid size",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{48 << 1, 0x00, 0x00, 0x05, 0x00},
				},
			},
			"invalid aggregation unit (invalid size)",
		},
		{
			"aggregation unit no NALUs",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{48 << 1, 0x00, 0x00, 0x00},
				},
			},
			"aggregation unit doesn't contain any NALU",
		},
		{
			"fragmentation unit invalid",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{49 << 1, 0x00},
				},
			},
			"payload is too short",
		},
		{
			"fragmentation unit start and end bit",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{49 << 1, 0x00, 0b11000000},
				},
			},
			"invalid fragmentation unit (can't contain both a start and end bit)",
		},
		{
			"fragmentation unit non-starting 1",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{49 << 1, 0x00, 0b01000000},
				},
			},
			"received a non-starting fragmentation unit without any previous fragmentation units",
		},
		{
			"fragmentation unit non-starting 2",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{0x01, 0x00},
				},
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{49 << 1, 0x00, 0b00000000},
				},
			},
			"invalid fragmentation unit (non-starting)",
		},
		{
			"paci",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17645,
						Timestamp:      2289527317,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{50 << 1, 0x00},
				},
			},
			"PACI packets are not supported (yet)",
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			d.Init()

			var lastErr error
			for _, pkt := range ca.pkts {
				_, _, lastErr = d.Decode(pkt)
			}
			require.EqualError(t, lastErr, ca.err)
		})
	}
}
