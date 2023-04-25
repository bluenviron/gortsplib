//go:build go1.18
// +build go1.18

package rtpvp9

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

			var frame []byte
			var err error

			for _, pkt := range ca.pkts {
				frame, _, err = d.Decode(pkt)
			}

			require.NoError(t, err)
			require.Equal(t, ca.frame, frame)
		})
	}
}

func FuzzDecoder(f *testing.F) {
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
