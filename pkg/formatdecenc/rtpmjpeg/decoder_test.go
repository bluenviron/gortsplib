//go:build go1.18
// +build go1.18

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
				image, pts, err := d.Decode(pkt)
				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
				require.Equal(t, ca.pts, pts)
				require.Equal(t, ca.image, image)
			}
		})
	}
}

func FuzzDecoderUnmarshal(f *testing.F) {
	d := &Decoder{}
	d.Init()

	f.Fuzz(func(t *testing.T, b []byte, m bool) {
		d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         m,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: b,
		})
	})
}
