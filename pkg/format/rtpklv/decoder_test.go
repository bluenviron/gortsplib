package rtpklv

import (
	"errors"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			var klvUnit []byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				addUnits, err := d.Decode(pkt)

				// test input integrity
				require.Equal(t, clone, pkt)

				if errors.Is(err, ErrMorePacketsNeeded) {
					continue
				}

				require.NoError(t, err)
				klvUnit = append(klvUnit, addUnits...)
			}

			require.Equal(t, ca.klvUnit, klvUnit)
		})
	}
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, am bool, b []byte, bm bool) {
		d := &Decoder{}
		err := d.Init()
		require.NoError(t, err)

		klvUnit, err := d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Marker:         am,
				SequenceNumber: 17645,
			},
			Payload: a,
		})

		if errors.Is(err, ErrMorePacketsNeeded) {
			klvUnit, err = d.Decode(&rtp.Packet{
				Header: rtp.Header{
					Marker:         bm,
					SequenceNumber: 17646,
				},
				Payload: b,
			})
		}

		if err == nil {
			if len(klvUnit) == 0 {
				t.Errorf("should not happen")
			}
		}
	})
}
