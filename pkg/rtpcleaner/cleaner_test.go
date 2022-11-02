package rtpcleaner

import (
	"bytes"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestRemovePadding(t *testing.T) {
	cleaner := New(false, false)

	out, err := cleaner.Process(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			Marker:         true,
			SequenceNumber: 34572,
			Padding:        true,
		},
		Payload:     bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 64/4),
		PaddingSize: 64,
	})
	require.NoError(t, err)
	require.Equal(t, []*Output{{
		Packet: &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    96,
				Marker:         true,
				SequenceNumber: 34572,
			},
			Payload: bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04}, 64/4),
		},
		PTSEqualsDTS: true,
	}}, out)
}

func TestH264ProcessEvenIfInvalid(t *testing.T) {
	cleaner := New(true, true)

	out, err := cleaner.Process(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			Marker:         false,
			SequenceNumber: 34572,
		},
		Payload: []byte{25},
	})
	require.Error(t, err)
	require.Equal(t, []*Output{{
		Packet: &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    96,
				Marker:         false,
				SequenceNumber: 34572,
			},
			Payload: []byte{25},
		},
	}}, out)
}

func TestH264RandomAccess(t *testing.T) {
	for _, ca := range []string{
		"standard",
	} {
		t.Run(ca, func(t *testing.T) {
			cleaner := New(true, true)

			var payload []byte
			if ca == "standard" {
				payload = append([]byte{0x1C, 1 << 6},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 10/5)...)
			}

			out, err := cleaner.Process(&rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    96,
					SequenceNumber: 34572,
				},
				Payload: payload,
			})
			require.NoError(t, err)

			if ca == "standard" {
				require.Equal(t, []*Output{{
					Packet: &rtp.Packet{
						Header: rtp.Header{
							Version:        2,
							PayloadType:    96,
							SequenceNumber: 34572,
						},
						Payload: payload,
					},
				}}, out)
			}
		})
	}
}
