//go:build go1.18
// +build go1.18

package rtpsimpleaudio

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{
				SampleRate: 8000,
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
				Payload: []byte{0x01, 0x02, 0x03, 0x04},
			}
			_, _, err := d.Decode(&pkt)
			require.NoError(t, err)

			frame, pts, err := d.Decode(ca.pkt)
			require.NoError(t, err)
			require.Equal(t, ca.pts, pts)

			require.Equal(t, ca.frame, frame)
		})
	}
}
