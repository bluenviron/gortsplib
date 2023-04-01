//go:build go1.18
// +build go1.18

package rtplpcm

import (
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{
				BitDepth:     24,
				SampleRate:   48000,
				ChannelCount: 2,
			}
			d.Init()

			// send an initial packet downstream
			// in order to compute the right timestamp,
			// that is relative to the initial packet
			pkt := rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    0,
					SequenceNumber: 17645,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			}
			_, _, err := d.Decode(&pkt)
			require.NoError(t, err)

			var samples []byte
			expPTS := ca.pts

			for _, pkt := range ca.pkts {
				partial, pts, err := d.Decode(pkt)
				require.NoError(t, err)
				require.Equal(t, expPTS, pts)
				samples = append(samples, partial...)
				expPTS += time.Duration(len(partial)/(24*2/8)) * time.Second / 48000
			}

			require.Equal(t, ca.samples, samples)
		})
	}
}

func FuzzDecoderUnmarshal(f *testing.F) {
	d := &Decoder{
		BitDepth:     24,
		SampleRate:   48000,
		ChannelCount: 2,
	}
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
