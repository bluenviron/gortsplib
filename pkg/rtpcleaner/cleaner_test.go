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

func TestGenericOversized(t *testing.T) {
	cleaner := New(false, true)

	_, err := cleaner.Process(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			Marker:         false,
			SequenceNumber: 34572,
		},
		Payload: bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 2050/5),
	})
	require.EqualError(t, err, "payload size (2062) is greater than maximum allowed (1472)")
}

func TestH264Oversized(t *testing.T) {
	cleaner := New(true, true)

	out, err := cleaner.Process(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			Marker:         false,
			SequenceNumber: 34572,
		},
		Payload: append(
			[]byte{0x1C, 1<<7 | 0x05},
			bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 2050/5)...,
		),
	})
	require.NoError(t, err)
	require.Equal(t, []*Output(nil), out)

	out, err = cleaner.Process(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			PayloadType:    96,
			Marker:         true,
			SequenceNumber: 34573,
		},
		Payload: append(
			[]byte{0x1C, 1 << 6},
			bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 2050/5)...,
		),
	})
	require.NoError(t, err)
	require.Equal(t, []*Output{
		{
			Packet: &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    96,
					Marker:         false,
					SequenceNumber: 34572,
				},
				Payload: append(
					append(
						[]byte{0x1c, 0x85},
						bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 291)...,
					),
					[]byte{0x01, 0x02, 0x03}...,
				),
			},
		},
		{
			Packet: &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    96,
					Marker:         false,
					SequenceNumber: 34573,
				},
				Payload: append(
					append(
						[]byte{0x1c, 0x05, 0x04, 0x05},
						bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 291)...,
					),
					[]byte{0x01}...,
				),
			},
		},
		{
			Packet: &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    96,
					Marker:         true,
					SequenceNumber: 34574,
				},
				Payload: append(
					[]byte{0x1c, 0x45, 0x02, 0x03, 0x04, 0x05},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 236)...,
				),
			},
			H264NALUs: [][]byte{
				append(
					[]byte{0x05},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 4100/5)...,
				),
			},
			PTSEqualsDTS: true,
		},
	}, out)
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
		"oversized",
	} {
		t.Run(ca, func(t *testing.T) {
			cleaner := New(true, true)

			var payload []byte
			if ca == "standard" {
				payload = append([]byte{0x1C, 1 << 6},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 10/5)...)
			} else {
				payload = append([]byte{0x1C, 1 << 6},
					bytes.Repeat([]byte{0x01, 0x02, 0x03, 0x04, 0x05}, 2048/5)...)
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
			} else {
				require.Equal(t, []*Output(nil), out)
			}
		})
	}
}
