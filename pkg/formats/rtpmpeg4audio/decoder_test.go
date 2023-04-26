package rtpmpeg4audio

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{
				SampleRate:       48000,
				SizeLength:       ca.sizeLength,
				IndexLength:      ca.indexLength,
				IndexDeltaLength: ca.indexDeltaLength,
			}
			d.Init()

			var aus [][]byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				addAUs, _, err := d.Decode(pkt)
				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
				aus = append(aus, addAUs...)

				// test input integrity
				require.Equal(t, clone, pkt)
			}

			require.Equal(t, ca.aus, aus)
		})
	}
}

func TestDecodeADTS(t *testing.T) {
	d := &Decoder{
		SampleRate:       16000,
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}
	d.Init()

	for i := 0; i < 2; i++ {
		aus, _, err := d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289526357,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{
				0x00, 0x10, 0x00, 0x09 << 3,
				0xff, 0xf1, 0x4c, 0x80, 0x1, 0x3f, 0xfc, 0xaa, 0xbb,
			},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{{0xaa, 0xbb}}, aus)
	}
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, am bool, b []byte, bm bool) {
		d := &Decoder{
			SampleRate:       16000,
			SizeLength:       13,
			IndexLength:      3,
			IndexDeltaLength: 3,
		}
		d.Init()

		d.Decode(&rtp.Packet{
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

		d.Decode(&rtp.Packet{
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
