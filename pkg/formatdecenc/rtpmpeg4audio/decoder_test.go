//go:build go1.18
// +build go1.18

package rtpmpeg4audio

import (
	"testing"
	"time"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/aler9/gortsplib/v2/pkg/codecs/mpeg4audio"
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
			}

			switch {
			case ca.sizeLength == 13 && ca.indexLength == 3:
				pkt.Payload = []byte{0x00, 0x10, 0x00, 0x08, 0x0}

			case ca.sizeLength == 13 && ca.indexLength == 0:
				pkt.Payload = []byte{0x00, 0x0d, 0x00, 0x08, 0x0}

			case ca.sizeLength == 6:
				pkt.Payload = []byte{0x00, 0x08, 0x04, 0x0}

			case ca.sizeLength == 21:
				pkt.Payload = []byte{0x00, 0x18, 0x00, 0x0, 0x08, 0x00}
			}

			_, _, err := d.Decode(&pkt)
			require.NoError(t, err)

			var aus [][]byte
			expPTS := ca.pts

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				addAUs, pts, err := d.Decode(pkt)
				if err == ErrMorePacketsNeeded {
					continue
				}

				require.NoError(t, err)
				require.Equal(t, expPTS, pts)
				aus = append(aus, addAUs...)
				expPTS += time.Duration(len(aus)) * mpeg4audio.SamplesPerAccessUnit * time.Second / 48000

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

func FuzzDecoderUnmarshal(f *testing.F) {
	d := &Decoder{
		SampleRate:       16000,
		SizeLength:       13,
		IndexLength:      3,
		IndexDeltaLength: 3,
	}
	d.Init()

	f.Fuzz(func(t *testing.T, b []byte, m bool) {
		d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         m,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: b,
		})
	})
}
