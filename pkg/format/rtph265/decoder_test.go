package rtph265

import (
	"errors"
	"testing"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h265"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			var nalus [][]byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				addNALUs, err := d.Decode(pkt)

				// test input integrity
				require.Equal(t, clone, pkt)

				if errors.Is(err, ErrMorePacketsNeeded) {
					continue
				}

				require.NoError(t, err)
				nalus = append(nalus, addNALUs...)
			}

			require.Equal(t, ca.nalus, nalus)
		})
	}
}

func TestDecoderErrorLimit(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	for i := 0; i <= h265.MaxNALUsPerAccessUnit; i++ {
		_, err = d.Decode(&rtp.Packet{
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

	require.EqualError(t, err, "NALU count (22) exceeds maximum allowed (21)")
}

func TestDecodeErrorMissingPacket(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	_, err = d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         false,
			PayloadType:    96,
			SequenceNumber: 17645,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x63, 0x02, 0x80, 0x03, 0x04},
	})
	require.Equal(t, ErrMorePacketsNeeded, err)

	_, err = d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         false,
			PayloadType:    96,
			SequenceNumber: 17647,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x63, 0x02, 0x00, 0x04},
	})
	require.EqualError(t, err, "discarding frame since a RTP packet is missing")
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, am bool, b []byte, bm bool) {
		d := &Decoder{}
		err := d.Init()
		require.NoError(t, err)

		au, err := d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Marker:         am,
				SequenceNumber: 17645,
			},
			Payload: a,
		})

		if errors.Is(err, ErrMorePacketsNeeded) {
			au, err = d.Decode(&rtp.Packet{
				Header: rtp.Header{
					Marker:         bm,
					SequenceNumber: 17646,
				},
				Payload: b,
			})
		}

		if err == nil {
			if len(au) == 0 {
				t.Errorf("should not happen")
			}

			for _, nalu := range au {
				if len(nalu) == 0 {
					t.Errorf("should not happen")
				}
			}
		}
	})
}
