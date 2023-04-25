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

			frames, _, err := d.Decode(ca.pkt)
			require.NoError(t, err)
			require.Equal(t, ca.frames, frames)
		})
	}
}

func FuzzDecoder(f *testing.F) {
	d := &Decoder{}
	d.Init()

	f.Fuzz(func(t *testing.T, b []byte) {
		d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: b,
		})
	})
}
