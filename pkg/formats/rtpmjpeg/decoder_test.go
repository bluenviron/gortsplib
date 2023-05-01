package rtpmjpeg

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			d.Init()

			for _, pkt := range ca.pkts {
				image, _, err := d.Decode(pkt)
				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
				require.Equal(t, ca.image, image)
			}
		})
	}
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, am bool, b []byte, bm bool) {
		d := &Decoder{}
		d.Init()

		d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         am,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: a,
		})

		d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         bm,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: b,
		})
	})
}
