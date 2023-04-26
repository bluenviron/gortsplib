package rtpmpeg2audio

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

			var frames [][]byte
			var err error

			for _, pkt := range ca.pkts {
				frames, _, err = d.Decode(pkt)
			}

			require.NoError(t, err)
			require.Equal(t, ca.frames, frames)
		})
	}
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, b []byte) {
		d := &Decoder{}
		d.Init()

		d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    14,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: a,
		})

		d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    14,
				SequenceNumber: 17646,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: b,
		})
	})
}
