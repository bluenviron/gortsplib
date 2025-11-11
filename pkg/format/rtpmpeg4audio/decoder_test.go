package rtpmpeg4audio

import (
	"bytes"
	"errors"
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecodeGeneric(t *testing.T) {
	for _, ca := range casesGeneric {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{
				SizeLength:       ca.sizeLength,
				IndexLength:      ca.indexLength,
				IndexDeltaLength: ca.indexDeltaLength,
			}
			err := d.Init()
			require.NoError(t, err)

			var aus [][]byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				var addAUs [][]byte
				addAUs, err = d.Decode(pkt)

				// test input integrity
				require.Equal(t, clone, pkt)

				if errors.Is(err, ErrMorePacketsNeeded) {
					continue
				}

				require.NoError(t, err)
				aus = append(aus, addAUs...)
			}

			require.Equal(t, ca.aus, aus)
		})
	}
}

func TestDecodeGenericADTS(t *testing.T) {
	d := &Decoder{
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}
	err := d.Init()
	require.NoError(t, err)

	for range 2 {
		var aus [][]byte
		aus, err = d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 17645,
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

func TestDecodeGenericErrorMissingPacket(t *testing.T) {
	d := &Decoder{
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}
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
			[]byte{0x0, 0x10, 0x2d, 0x80},
			bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
		),
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
			[]byte{0x00, 0x10, 0x2d, 0x80},
			bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
		),
	})
	require.EqualError(t, err, "discarding frame since a RTP packet is missing")
}

func FuzzDecoderGeneric(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, am bool, b []byte, bm bool) {
		d := &Decoder{
			SizeLength:       13,
			IndexLength:      3,
			IndexDeltaLength: 3,
		}
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
				SequenceNumber: 17646,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: b,
		})
	})
}
