//go:build go1.18
// +build go1.18

package rtpvp9

import (
	"testing"
	"time"

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
				Payload: []byte{0x9c, 0xb5, 0xaf, 0x01, 0x02, 0x03, 0x04},
			}
			_, _, err := d.Decode(&pkt)
			require.NoError(t, err)

			var frame []byte

			for _, pkt := range ca.pkts {
				var pts time.Duration
				frame, pts, err = d.Decode(pkt)
				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
				require.Equal(t, ca.pts, pts)
			}

			require.Equal(t, ca.frame, frame)
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
