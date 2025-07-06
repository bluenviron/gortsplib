package rtpklv

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			var klvUnit [][]byte

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
