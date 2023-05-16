package rtph265

import (
	"testing"

	"github.com/bluenviron/mediacommon/pkg/codecs/h265"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			d.Init()

			var nalus [][]byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				addNALUs, _, err := d.Decode(pkt)
				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
				nalus = append(nalus, addNALUs...)

				// test input integrity
				require.Equal(t, clone, pkt)
			}

			require.Equal(t, ca.nalus, nalus)
		})
	}
}

func TestDecoderErrorLimit(t *testing.T) {
	d := &Decoder{}
	d.Init()
	var err error

	for i := 0; i <= h265.MaxNALUsPerGroup; i++ {
		_, _, err = d.DecodeUntilMarker(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{1, 2, 3, 4},
		})
	}

	require.EqualError(t, err, "NALU count exceeds maximum allowed (20)")
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, b []byte) {
		d := &Decoder{}
		d.Init()

		d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: a,
		})

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
