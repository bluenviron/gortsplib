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
	}

	r := &Reorderer{}
	r.Initialize()
	r.absPos = 40

	for _, entry := range sequence {
		out, missing := r.Process(entry.in)
		require.Equal(t, entry.out, out)
		require.Equal(t, uint(0), missing)
	}
}

func TestBufferIsFull(t *testing.T) {
	r := &Reorderer{}
	r.Initialize()
	r.absPos = 25
	sn := uint16(1564)
	toMiss := uint(34)

	out, missing := r.Process(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: sn,
		},
	})
	require.Equal(t, []*rtp.Packet{{
		Header: rtp.Header{
			SequenceNumber: sn,
		},
	}}, out)
	require.Equal(t, uint(0), missing)
	sn++

	var expected []*rtp.Packet

	for i := uint(0); i < 64-toMiss; i++ {
		out, missing = r.Process(&rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: sn + uint16(toMiss),
			},
		})
		require.Equal(t, []*rtp.Packet(nil), out)
		require.Equal(t, uint(0), missing)

		expected = append(expected, &rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: sn + uint16(toMiss),
			},
		})
		sn++
	}

	out, missing = r.Process(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: sn + uint16(toMiss),
		},
	})
	require.Equal(t, toMiss, missing)

	expected = append(expected, &rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: sn + uint16(toMiss),
		},
	})

	require.Equal(t, expected, out)
}

func TestReset(t *testing.T) {
	r := &Reorderer{}
	r.Initialize()
	sn := uint16(1234)

	r.Process(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: sn,
		},
	})

	sn = 0xF234
	for i := 0; i < 64; i++ {
		out, missing := r.Process(&rtp.Packet{
			Header: rtp.Header{
				SequenceNumber: sn,
			},
		})
		require.Equal(t, []*rtp.Packet(nil), out)
		require.Equal(t, uint(0), missing)
		sn++
	}

	out, missing := r.Process(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: sn,
		},
	})
	require.Equal(t, []*rtp.Packet{{
		Header: rtp.Header{
			SequenceNumber: sn,
		},
	}}, out)
	require.Equal(t, uint(0), missing)
}

func TestCustomBufferSize(t *testing.T) {
	customSize := 128
	r := &Reorderer{
		BufferSize: customSize,
	}
	r.Initialize()

	// Set absPos to an arbitrary value.
	r.absPos = 10

	// Process first packet; behaves as usual.
	firstSeq := uint16(50)
	out, missing := r.Process(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: firstSeq,
		},
	})
	require.Equal(t, []*rtp.Packet{{
		Header: rtp.Header{
			SequenceNumber: firstSeq,
		},
	}}, out)
	require.Equal(t, uint(0), missing)

	// At this point, expectedSeqNum == firstSeq + 1 (i.e. 51).
	// Now, send a packet with a gap larger than the custom buffer size.
	// For BufferSize = 128, let's send a packet with SequenceNumber = 51 + 130 = 181.
	nextSeq := uint16(181)
	out, missing = r.Process(&rtp.Packet{
		Header: rtp.Header{
			SequenceNumber: nextSeq,
		},
	})
	// Since there are no packets buffered, n remains 1.
	// relPos = 181 - 51 = 130; so missing should be 130
	require.Equal(t, uint(130), missing)
	require.Equal(t, []*rtp.Packet{{
		Header: rtp.Header{
			SequenceNumber: nextSeq,
		},
	}}, out)
}
