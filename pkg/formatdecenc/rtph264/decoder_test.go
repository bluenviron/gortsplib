//go:build go1.18
// +build go1.18

package rtph264

import (
	"bytes"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

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

func TestDecodeCorruptedFragment(t *testing.T) {
	d := &Decoder{}
	d.Init()

	_, _, err := d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         false,
			PayloadType:    96,
			SequenceNumber: 17645,
			Timestamp:      2289527317,
			SSRC:           0x9dbb7812,
		},
		Payload: mergeBytes(
			[]byte{
				0x1c, 0x85,
			},
			bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
			[]byte{0x00, 0x01},
		),
	})
	require.Equal(t, ErrMorePacketsNeeded, err)

	nalus, _, err := d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         false,
			PayloadType:    96,
			SequenceNumber: 17646,
			Timestamp:      2289527317,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x01, 0x00},
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x01, 0x00}}, nalus)
}

func TestDecodeSTAPAWithPadding(t *testing.T) {
	d := &Decoder{}
	d.Init()

	pkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17645,
			Timestamp:      2289526357,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{
			0x18, 0x00, 0x02, 0xaa,
			0xbb, 0x00, 0x02, 0xcc, 0xdd, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}

	nalus, _, err := d.Decode(&pkt)
	require.NoError(t, err)
	require.Equal(t, [][]byte{
		{0xaa, 0xbb},
		{0xcc, 0xdd},
	}, nalus)
}

func TestDecodeAnnexB(t *testing.T) {
	d := &Decoder{}
	d.Init()

	for i := 0; i < 2; i++ {
		nalus, _, err := d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 17647,
				Timestamp:      2289531307,
				SSRC:           0x9dbb7812,
			},
			Payload: mergeBytes(
				[]byte{0x00, 0x00, 0x00, 0x01},
				[]byte{0x01, 0x02, 0x03, 0x04},
				[]byte{0x00, 0x00, 0x00, 0x01},
				[]byte{0x01, 0x02, 0x03, 0x04},
			),
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{
			{0x01, 0x02, 0x03, 0x04},
			{0x01, 0x02, 0x03, 0x04},
		}, nalus)
	}
}

func TestDecodeUntilMarker(t *testing.T) {
	d := &Decoder{}
	d.Init()

	nalus, _, err := d.DecodeUntilMarker(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         false,
			PayloadType:    96,
			SequenceNumber: 17647,
			Timestamp:      2289531307,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x01, 0x02},
	})
	require.Equal(t, ErrMorePacketsNeeded, err)
	require.Equal(t, [][]byte(nil), nalus)

	nalus, _, err = d.DecodeUntilMarker(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17647,
			Timestamp:      2289531307,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x01, 0x02},
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x01, 0x02}, {0x01, 0x02}}, nalus)
}

func FuzzDecoderUnmarshal(f *testing.F) {
	d := &Decoder{}
	d.Init()

	f.Fuzz(func(t *testing.T, b []byte) {
		d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: b,
		})
	})
}
