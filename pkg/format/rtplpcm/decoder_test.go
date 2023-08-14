package rtplpcm

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{
				BitDepth:     24,
				ChannelCount: 2,
			}
			err := d.Init()
			require.NoError(t, err)

			var samples []byte

			for _, pkt := range ca.pkts {
				partial, err := d.Decode(pkt)
				require.NoError(t, err)
				samples = append(samples, partial...)
			}

			require.Equal(t, ca.samples, samples)
		})
	}
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		d := &Decoder{
			BitDepth:     24,
			ChannelCount: 2,
		}
		d.Init() //nolint:errcheck

		d.Decode(&rtp.Packet{ //nolint:errcheck
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
