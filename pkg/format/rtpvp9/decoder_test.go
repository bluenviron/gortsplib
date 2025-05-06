package rtpvp9

import (
	"bytes"
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

			var frame []byte

			for _, pkt := range ca.pkts {
				frame, err = d.Decode(pkt)
			}

			require.NoError(t, err)
			require.Equal(t, ca.frame, frame)
		})
	}
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
		Payload: mergeBytes(
			[]byte{
				0x8b, 0xb5, 0xaf, 0x18, 0x07, 0x80, 0x03, 0x24,
				0x01, 0x14, 0x01, 0x82, 0x49, 0x83, 0x42, 0x00,
				0x77, 0xf0, 0x32, 0x34,
			},
			bytes.Repeat([]byte{1, 2, 3, 4}, 360)),
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
		Payload: mergeBytes(
			[]byte{0x81, 0xb5, 0xaf},
			bytes.Repeat([]byte{1, 2, 3, 4}, 364), []byte{1}),
	})
	require.EqualError(t, err, "discarding frame since a RTP packet is missing")
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, am bool, b []byte, bm bool) {
		d := &Decoder{}
		err := d.Init()
		require.NoError(t, err)

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
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: b,
		})
	})
}
