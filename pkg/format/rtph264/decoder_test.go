package rtph264

import (
	"bytes"
	"errors"
	"testing"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
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

func TestDecodeCorruptedFragment(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	_, err = d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17645,
			Timestamp:      2289527317,
			SSRC:           0x9dbb7812,
		},
		Payload: mergeBytes(
			[]byte{
				0x1c, 0x85,
			},
			bytes.Repeat([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 182),
			[]byte{0x00, 0x01},
		),
	})
	require.Equal(t, ErrMorePacketsNeeded, err)

	nalus, err := d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17646,
			Timestamp:      2289527317,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x01, 0x00},
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x01, 0x00}}, nalus)
}

func TestDecodeNoncompliantFragment(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	nalus, err := d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 18853,
			Timestamp:      1731630255,
			SSRC:           0x466b0000,
		},

		// FU-A with both start and end bit intentionally set
		// While not compliant with RFC 6184, IP cameras from some vendors
		// (e.g. CostarHD) have been observed to produce such FU-A payloads for
		// sufficiently small P-frames.
		Payload: mergeBytes(
			[]byte{
				0x3c,       // FU indicator
				0xc1,       // FU header (start and end bit both intentionally set)
				0xe7, 0x00, // DON
				0xca, 0xfe, // Payload
			},
		),
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x21, 0xe7, 0x00, 0xca, 0xfe}}, nalus)
}

func TestDecodeSTAPAWithPadding(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	pkt := rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17645,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{
			0x18, 0x00, 0x02, 0xaa,
			0xbb, 0x00, 0x02, 0xcc, 0xdd, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		},
	}

	nalus, err := d.Decode(&pkt)
	require.NoError(t, err)
	require.Equal(t, [][]byte{
		{0xaa, 0xbb},
		{0xcc, 0xdd},
	}, nalus)
}

func TestDecodeAnnexB(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	nalus, err := d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17647,
			Timestamp:      2289531307,
			SSRC:           0x9dbb7812,
		},
		Payload: mergeBytes(
			[]byte{0x01, 0x02, 0x03, 0x04},
		),
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{
		{0x01, 0x02, 0x03, 0x04},
	}, nalus)

	for i := 0; i < 2; i++ {
		nalus, err := d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 17647,
				Timestamp:      2289531307,
				SSRC:           0x9dbb7812,
			},
			Payload: mergeBytes(
				[]byte{0x00, 0x00, 0x00, 0x01},
				[]byte{0x01, 0x02, 0x03, 0x04},
				[]byte{0x00, 0x00, 0x00, 0x01},
				[]byte{0x01, 0x02, 0x03, 0x04},
			),
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{
			{0x01, 0x02, 0x03, 0x04},
			{0x01, 0x02, 0x03, 0x04},
		}, nalus)
	}
}

func TestDecodeAccessUnit(t *testing.T) {
	for _, ca := range []struct {
		name string
		pkts []*rtp.Packet
		au   [][]byte
	}{
		{
			"marker-splitted",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         false,
						PayloadType:    96,
						SequenceNumber: 17647,
						Timestamp:      2289531307,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{1, 2},
				},
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         true,
						PayloadType:    96,
						SequenceNumber: 17647,
						Timestamp:      2289531307,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{3, 4},
				},
			},
			[][]byte{{1, 2}, {3, 4}},
		},
		{
			"timestamp-splitted (FLIR M400)",
			[]*rtp.Packet{
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         false,
						PayloadType:    96,
						SequenceNumber: 17647,
						Timestamp:      2289531307,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{1, 2},
				},
				{
					Header: rtp.Header{
						Version:        2,
						Marker:         false,
						PayloadType:    96,
						SequenceNumber: 17647,
						Timestamp:      2289531308,
						SSRC:           0x9dbb7812,
					},
					Payload: []byte{3, 4},
				},
			},
			[][]byte{{1, 2}},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			var au [][]byte

			for i, pkt := range ca.pkts {
				au, err = d.Decode(pkt)
				if i != len(ca.pkts)-1 {
					require.Equal(t, ErrMorePacketsNeeded, err)
				} else {
					require.NoError(t, err)
					require.Equal(t, ca.au, au)
				}
			}
		})
	}
}

func TestDecoderErrorNALUSize(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	size := 0
	i := uint16(0)

	for size < h264.MaxAccessUnitSize {
		flags := byte(0)
		if size == 0 {
			flags = 0b10000000
		}

		_, err = d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17645 + i,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: append(
				[]byte{byte(h264.NALUTypeFUA), flags},
				bytes.Repeat([]byte{1, 2, 3, 4}, 1400/4)...,
			),
		})

		size += 1400
		i++
	}

	require.EqualError(t, err, "NALU size (8388801) is too big, maximum is 8388608")
}

func TestDecoderErrorNALUCount(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	for i := 0; i <= h264.MaxNALUsPerAccessUnit; i++ {
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

	require.EqualError(t, err, "NALU count (26) exceeds maximum allowed (25)")
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
		Payload: []byte{0x1c, 0x85, 0x01, 0x02},
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
		Payload: []byte{0x1c, 0x05, 0x01, 0x02},
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
