package rtpopus

import (
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

var cases = []struct {
	name string
	op   []byte
	pts  time.Duration
	pkt  *rtp.Packet
}{
	{
		"a",
		[]byte{0x01, 0x02, 0x03, 0x04},
		20 * time.Millisecond,
		&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x01, 0x02, 0x03, 0x04},
		},
	},
}

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{
				SampleRate: 48000,
			}
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
				Payload: []byte{0x00},
			}

			_, _, err := d.Decode(&pkt)
			require.NoError(t, err)

			op, pts, err := d.Decode(ca.pkt)
			require.NoError(t, err)
			require.Equal(t, ca.op, op)
			require.Equal(t, ca.pts, pts)
		})
	}
}
