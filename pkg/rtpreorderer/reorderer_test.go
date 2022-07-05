package rtpreorderer

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestReorder(t *testing.T) {
	sequence := []struct {
		in  *rtp.Packet
		out []*rtp.Packet
	}{
		{
			// first packet
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65530,
				},
			},
			[]*rtp.Packet{{
				Header: rtp.Header{
					SequenceNumber: 65530,
				},
			}},
		},
		{
			// packet sent before first packet
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65529,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// ok
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65531,
				},
			},
			[]*rtp.Packet{{
				Header: rtp.Header{
					SequenceNumber: 65531,
				},
			}},
		},
		{
			// duplicated
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65531,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// gap
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65535,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// unordered
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65533,
					PayloadType:    96,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// unordered + duplicated
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65533,
					PayloadType:    97,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// unordered
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65532,
				},
			},
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						SequenceNumber: 65532,
					},
				},
				{
					Header: rtp.Header{
						SequenceNumber: 65533,
						PayloadType:    96,
					},
				},
			},
		},
		{
			// unordered
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 65534,
				},
			},
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						SequenceNumber: 65534,
					},
				},
				{
					Header: rtp.Header{
						SequenceNumber: 65535,
					},
				},
			},
		},
		{
			// overflow + gap
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 1,
				},
			},
			[]*rtp.Packet(nil),
		},
		{
			// unordered
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 0,
				},
			},
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						SequenceNumber: 0,
					},
				},
				{
					Header: rtp.Header{
						SequenceNumber: 1,
					},
				},
			},
		},
		{
			// buffer is full
			&rtp.Packet{
				Header: rtp.Header{
					SequenceNumber: 67,
				},
			},
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						SequenceNumber: 67,
					},
				},
			},
		},
	}

	r := New()
	r.absPos = 40

	for _, entry := range sequence {
		out := r.Process(entry.in)
		require.Equal(t, entry.out, out)
	}
}
