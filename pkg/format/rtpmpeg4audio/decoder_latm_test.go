package rtpmpeg4audio

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecodeLATM(t *testing.T) {
	for _, ca := range casesLATM {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{
				LATM: true,
			}
			err := d.Init()
			require.NoError(t, err)

			var aus [][]byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				aus, err = d.Decode(pkt)

				// test input integrity
				require.Equal(t, clone, pkt)

				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
			}

			require.Equal(t, ca.au, aus[0])
		})
	}
}

func TestDecodeLATMOtherData(t *testing.T) {
	d := &Decoder{
		LATM: true,
	}
	err := d.Init()
	require.NoError(t, err)

	aus, err := d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17645,
			SSRC:           2646308882,
		},
		Payload: []byte{
			0x04, 0x01, 0x02, 0x03, 0x04, 5, 6,
		},
	})
	require.NoError(t, err)

	require.Equal(t, []byte{1, 2, 3, 4}, aus[0])
}

func FuzzDecoderLATM(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, am bool, b []byte, bm bool) {
		d := &Decoder{
			LATM: true,
		}
		d.Init() //nolint:errcheck

		d.Decode(&rtp.Packet{ //nolint:errcheck
			Header: rtp.Header{
				Version:        2,
				Marker:         am,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: a,
		})

		d.Decode(&rtp.Packet{ //nolint:errcheck
			Header: rtp.Header{
				Version:        2,
				Marker:         bm,
				PayloadType:    96,
				SequenceNumber: 17646,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: b,
		})
	})
}
