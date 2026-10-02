package rtph264_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/gortsplib/v5/pkg/format/rtph264"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &rtph264.Decoder{PacketizationMode: 1}
			err := d.Init()
			require.NoError(t, err)

			var au [][]byte

			for _, pkt := range ca.pkts {
				clone := pkt.Clone()

				var addNALUs [][]byte
				addNALUs, err = d.Decode(pkt)

				// test input integrity
				require.Equal(t, clone, pkt)

				if errors.Is(err, rtph264.ErrMorePacketsNeeded) {
					continue
				}

				require.NoError(t, err)
				au = append(au, addNALUs...)
			}

			require.Equal(t, ca.au, au)
		})
	}
}

var casesDecodeOnly = []struct {
	name string
	pkts []*rtp.Packet
	au   [][]byte
}{
	{
		"corrupted fragment",
		[]*rtp.Packet{
			{
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
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289527317,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x01, 0x00},
			},
		},
		[][]byte{{0x01, 0x00}},
	},
	{
		"issue gortsplib/649 (CostarHD, FU-A with both start and end bit set)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 18853,
					Timestamp:      1731630255,
					SSRC:           0x466b0000,
				},
				Payload: mergeBytes(
					[]byte{
						0x3c,       // FU indicator
						0xc1,       // FU header (start and end bit both intentionally set)
						0xe7, 0x00, // DON
						0xca, 0xfe, // Payload
					},
				),
			},
		},
		[][]byte{{0x21, 0xe7, 0x00, 0xca, 0xfe}},
	},
	{
		"STAP-A with padding",
		[]*rtp.Packet{
			{
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
			},
		},
		[][]byte{
			{0xaa, 0xbb},
			{0xcc, 0xdd},
		},
	},
	{
		"issue gortsplib/199 (AnnexB-encoded streams, single NALU)",
		[]*rtp.Packet{
			{
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
			},
		},
		[][]byte{
			{0x01, 0x02, 0x03, 0x04},
		},
	},
	{
		"issue gortsplib/199 (AnnexB-encoded streams, multiple NALUs)",
		[]*rtp.Packet{
			{
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
			},
		},
		[][]byte{
			{0x01, 0x02, 0x03, 0x04},
			{0x01, 0x02, 0x03, 0x04},
		},
	},
	{
		"marker-splitted access units",
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
		"issue mediamtx/3945 (FLIR M400, timestamp-splitted access units)",
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
	{
		"issue gortsplib/989 (Amatek AR-N3222F, multiple NALUs in FU, basic)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 54972,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x7c, 0x87, 0x4d, 0x00, 0x33, 0x8a, 0x8a, 0x50,
					0x28, 0x02, 0xdd, 0x34, 0x40, 0x00, 0x00, 0xfa,
					0x00, 0x00, 0x30, 0xd4,
				},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 54973,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x7c, 0x47, 0x01, 0x00, 0x00, 0x00, 0x01, 0x68,
					0xee, 0x3c, 0x80,
				},
			},
		},
		[][]byte{
			{
				0x67, 0x4d, 0x00, 0x33, 0x8a, 0x8a, 0x50, 0x28,
				0x02, 0xdd, 0x34, 0x40, 0x00, 0x00, 0xfa, 0x00,
				0x00, 0x30, 0xd4, 0x01,
			},
			{
				0x68, 0xee, 0x3c, 0x80,
			},
		},
	},
	{
		"issue gortsplib/989 (Amatek AR-N3222F, multiple NALUs in FU, leading start code)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 54972,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x1c, 0x80, 0x00, 0x01, 0x67, 0x4d, 0x00, 0x33,
					0x8a, 0x8a, 0x50, 0x28, 0x02, 0xdd, 0x34, 0x40,
					0x00, 0x00, 0xfa, 0x00,
				},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 54973,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x1c, 0x40, 0x00, 0x30, 0xd4, 0x01, 0x00, 0x00,
					0x01, 0x68, 0xee, 0x3c, 0x80,
				},
			},
		},
		[][]byte{
			{
				0x67, 0x4d, 0x00, 0x33, 0x8a, 0x8a, 0x50, 0x28,
				0x02, 0xdd, 0x34, 0x40, 0x00, 0x00, 0xfa, 0x00,
				0x00, 0x30, 0xd4, 0x01,
			},
			{
				0x68, 0xee, 0x3c, 0x80,
			},
		},
	},
	{
		"issue gortsplib/989 (Amatek AR-N3222F, multiple NALUs in FU, leading and trailing start codes)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 54972,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x1c, 0x80, 0x00, 0x01, 0x67, 0x4d, 0x00, 0x33,
					0x8a, 0x8a, 0x50, 0x28, 0x02, 0xdd, 0x34, 0x40,
					0x00, 0x00, 0xfa, 0x00,
				},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 54973,
					SSRC:           0xda182e65,
				},
				Payload: []byte{
					0x1c, 0x40, 0x00, 0x30, 0xd4, 0x01, 0x00, 0x00,
					0x01, 0x68, 0xee, 0x3c, 0x80, 0x00, 0x00, 0x00,
					0x01,
				},
			},
		},
		[][]byte{
			{
				0x67, 0x4d, 0x00, 0x33, 0x8a, 0x8a, 0x50, 0x28,
				0x02, 0xdd, 0x34, 0x40, 0x00, 0x00, 0xfa, 0x00,
				0x00, 0x30, 0xd4, 0x01,
			},
			{
				0x68, 0xee, 0x3c, 0x80,
			},
		},
	},
	{
		"SEI with inner start code, fragmented (ONVIF Media Signing without emulation prevention)",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x1c, 0x86}, // FU-A, start, type 6
					[]byte{0x05, 0x18},
					[]byte{0x00, 0x5b, 0xc9, 0x3f, 0x2d, 0x71, 0x5e, 0x95, 0xad, 0xa4, 0x79, 0x6f, 0x90, 0x87, 0x7a, 0x6f},
					[]byte{0xaa, 0xbb, 0x00, 0x00},
				),
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: mergeBytes(
					[]byte{0x1c, 0x46}, // FU-A, end, type 6
					[]byte{0x00, 0x01, 0xcc, 0xdd, 0x80},
				),
			},
		},
		[][]byte{
			mergeBytes(
				[]byte{0x06, 0x05, 0x18},
				[]byte{0x00, 0x5b, 0xc9, 0x3f, 0x2d, 0x71, 0x5e, 0x95, 0xad, 0xa4, 0x79, 0x6f, 0x90, 0x87, 0x7a, 0x6f},
				[]byte{0xaa, 0xbb, 0x00, 0x00, 0x00, 0x01, 0xcc, 0xdd, 0x80},
			),
		},
	},
	{
		"SEI with inner 3-byte start code, fragmented",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x1c, 0x86, 0x05, 0x04, 0xaa, 0x00},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x1c, 0x46, 0x00, 0x01, 0xbb, 0x80},
			},
		},
		[][]byte{
			{0x06, 0x05, 0x04, 0xaa, 0x00, 0x00, 0x01, 0xbb, 0x80},
		},
	},
	{
		"SEI with inner start code, single packet, does not enable Annex-B mode",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x06, 0x05, 0x04, 0x00, 0x00, 0x00, 0x01, 0x80},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x06, 0x05, 0x04, 0x00, 0x00, 0x00, 0x01, 0x81},
			},
		},
		[][]byte{
			{0x06, 0x05, 0x04, 0x00, 0x00, 0x00, 0x01, 0x80},
			{0x06, 0x05, 0x04, 0x00, 0x00, 0x00, 0x01, 0x81},
		},
	},
	{
		"Annex-B packet with a leading start code and an SEI still enables Annex-B mode",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17647,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{
					0x00, 0x00, 0x00, 0x01, 0x06, 0x05, 0x04, 0xaa,
					0x00, 0x00, 0x00, 0x01, 0x65, 0xbb,
				},
			},
		},
		[][]byte{
			{0x06, 0x05, 0x04, 0xaa},
			{0x65, 0xbb},
		},
	},
	{
		"SEI is still split once a non-SEI NALU has enabled Annex-B mode",
		[]*rtp.Packet{
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         false,
					PayloadType:    96,
					SequenceNumber: 17645,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{
					0x00, 0x00, 0x00, 0x01, 0x65, 0xaa,
					0x00, 0x00, 0x00, 0x01, 0x65, 0xbb,
				},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    96,
					SequenceNumber: 17646,
					Timestamp:      2289531307,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{0x06, 0x05, 0x04, 0x00, 0x00, 0x00, 0x01, 0x80},
			},
		},
		[][]byte{
			{0x65, 0xaa},
			{0x65, 0xbb},
			{0x06, 0x05, 0x04},
			{0x80},
		},
	},
}

func TestDecodeOnly(t *testing.T) {
	for _, ca := range casesDecodeOnly {
		t.Run(ca.name, func(t *testing.T) {
			d := &rtph264.Decoder{PacketizationMode: 1}
			err := d.Init()
			require.NoError(t, err)

			var au [][]byte

			for i, pkt := range ca.pkts {
				au, err = d.Decode(pkt)

				if i != len(ca.pkts)-1 {
					require.ErrorIs(t, err, rtph264.ErrMorePacketsNeeded)
				} else {
					require.NoError(t, err)
				}
			}

			require.Equal(t, ca.au, au)
		})
	}
}

func TestDecodePacketizationMode0(t *testing.T) {
	d := &rtph264.Decoder{PacketizationMode: 0}
	err := d.Init()
	require.NoError(t, err)

	// single NALU
	au, err := d.Decode(&rtp.Packet{
		Header: rtp.Header{
			Version:        2,
			Marker:         true,
			PayloadType:    96,
			SequenceNumber: 17645,
			Timestamp:      2289527317,
			SSRC:           0x9dbb7812,
		},
		Payload: []byte{0x65, 0x88, 0x84, 0x00, 0x33},
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x65, 0x88, 0x84, 0x00, 0x33}}, au)
}

func TestDecodePacketizationMode0AllowsFUAAndSTAPA(t *testing.T) {
	t.Run("FU-A", func(t *testing.T) {
		d := &rtph264.Decoder{PacketizationMode: 0}
		err := d.Init()
		require.NoError(t, err)

		_, err = d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         false,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x7c, 0x85, 0xaa, 0xbb},
		})
		require.ErrorIs(t, err, rtph264.ErrMorePacketsNeeded)

		au, err := d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 17646,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x7c, 0x45, 0xcc, 0xdd},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{{0x65, 0xaa, 0xbb, 0xcc, 0xdd}}, au)
	})

	t.Run("STAP-A", func(t *testing.T) {
		d := &rtph264.Decoder{PacketizationMode: 0}
		err := d.Init()
		require.NoError(t, err)

		au, err := d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    96,
				SequenceNumber: 17645,
				Timestamp:      2289527317,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{0x18, 0x00, 0x02, 0xaa, 0xbb, 0x00, 0x03, 0xcc, 0xdd, 0xee},
		})
		require.NoError(t, err)
		require.Equal(t, [][]byte{{0xaa, 0xbb}, {0xcc, 0xdd, 0xee}}, au)
	})
}

func TestDecodeErrorNALUSize(t *testing.T) {
	d := &rtph264.Decoder{PacketizationMode: 1}
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

func TestDecodeErrorNALUCount(t *testing.T) {
	d := &rtph264.Decoder{PacketizationMode: 1}
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

	require.EqualError(t, err, "NALU count (51) exceeds maximum allowed (50)")
}

func TestDecodeErrorEmptyFUA(t *testing.T) {
	for _, ca := range []struct {
		name string
		pkts []*rtp.Packet
	}{
		{
			name: "single fragment",
			pkts: []*rtp.Packet{{
				Header:  rtp.Header{Marker: true},
				Payload: []byte{0x1c, 0xc0, 0x00, 0x00, 0x01},
			}},
		},
		{
			name: "multiple fragments",
			pkts: []*rtp.Packet{
				{Header: rtp.Header{SequenceNumber: 1}, Payload: []byte{0x1c, 0x80, 0x00, 0x00, 0x01}},
				{Header: rtp.Header{SequenceNumber: 2, Marker: true}, Payload: []byte{0x1c, 0x40}},
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			d := &rtph264.Decoder{PacketizationMode: 1}
			err := d.Init()
			require.NoError(t, err)

			for i, pkt := range ca.pkts {
				var au [][]byte
				au, err = d.Decode(pkt)
				require.Nil(t, au)
				if i != len(ca.pkts)-1 {
					require.ErrorIs(t, err, rtph264.ErrMorePacketsNeeded)
				} else {
					require.EqualError(t, err, "fragmented NALU doesn't contain any NALU")
				}
			}

			au, err := d.Decode(&rtp.Packet{
				Header:  rtp.Header{Marker: true},
				Payload: []byte{0x65, 0x88},
			})
			require.NoError(t, err)
			require.Equal(t, [][]byte{{0x65, 0x88}}, au)
		})
	}
}

func TestDecodeErrorEmptyFUAPreservesBufferedAU(t *testing.T) {
	d := &rtph264.Decoder{PacketizationMode: 1}
	err := d.Init()
	require.NoError(t, err)

	au, err := d.Decode(&rtp.Packet{
		Header:  rtp.Header{Timestamp: 1},
		Payload: []byte{0x65, 0x88},
	})
	require.Nil(t, au)
	require.ErrorIs(t, err, rtph264.ErrMorePacketsNeeded)

	au, err = d.Decode(&rtp.Packet{
		Header:  rtp.Header{SequenceNumber: 1, Timestamp: 1},
		Payload: []byte{0x1c, 0x80, 0x00, 0x00, 0x01},
	})
	require.Nil(t, au)
	require.ErrorIs(t, err, rtph264.ErrMorePacketsNeeded)

	au, err = d.Decode(&rtp.Packet{
		Header:  rtp.Header{SequenceNumber: 2, Timestamp: 1},
		Payload: []byte{0x1c, 0x40},
	})
	require.Nil(t, au)
	require.EqualError(t, err, "fragmented NALU doesn't contain any NALU")

	au, err = d.Decode(&rtp.Packet{
		Header:  rtp.Header{Timestamp: 2},
		Payload: []byte{0x41, 0x99},
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x65, 0x88}}, au)

	au, err = d.Decode(&rtp.Packet{
		Header:  rtp.Header{Timestamp: 2, Marker: true},
		Payload: []byte{0x41, 0xaa},
	})
	require.NoError(t, err)
	require.Equal(t, [][]byte{{0x41, 0x99}, {0x41, 0xaa}}, au)
}

func TestDecodeErrorMissingPacket(t *testing.T) {
	d := &rtph264.Decoder{PacketizationMode: 1}
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
	require.Equal(t, rtph264.ErrMorePacketsNeeded, err)

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

func TestDecodeTrailingSEI(t *testing.T) {
	picture := []byte{0x65, 0x88}
	sei := []byte{0x06, 0x05, 0x01}
	nonIDR := []byte{0x41, 0x9a}

	tests := []struct {
		name     string
		packets  []*rtp.Packet
		outcomes [][][]byte
		waiting  []bool
	}{
		{
			name: "trailing SEIs",
			packets: []*rtp.Packet{
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: picture},
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: sei},
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: sei},
				{Header: rtp.Header{Marker: true, Timestamp: 200}, Payload: nonIDR},
			},
			outcomes: [][][]byte{{picture}, nil, nil, {nonIDR}},
			waiting:  []bool{false, true, true, false},
		},
		{
			name: "different timestamp",
			packets: []*rtp.Packet{
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: picture},
				{Header: rtp.Header{Marker: true, Timestamp: 200}, Payload: sei},
				{Header: rtp.Header{Marker: true, Timestamp: 200}, Payload: sei},
			},
			outcomes: [][][]byte{{picture}, {sei}, {sei}},
			waiting:  []bool{false, false, false},
		},
		{
			name: "no previous picture",
			packets: []*rtp.Packet{
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: sei},
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: sei},
			},
			outcomes: [][][]byte{{sei}, {sei}},
			waiting:  []bool{false, false},
		},
		{
			name: "SEI before first picture at same timestamp",
			packets: []*rtp.Packet{
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: sei},
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: picture},
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: sei},
			},
			outcomes: [][][]byte{{sei}, {picture}, nil},
			waiting:  []bool{false, false, true},
		},
		{
			name: "non-SEI AU precedes SEI",
			packets: []*rtp.Packet{
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: picture},
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: []byte{0x09, 0x10}},
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: sei},
			},
			outcomes: [][][]byte{{picture}, {{0x09, 0x10}}, nil},
			waiting:  []bool{false, false, true},
		},
		{
			name: "mixed picture and SEI",
			packets: []*rtp.Packet{
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: picture},
				{Header: rtp.Header{Timestamp: 100}, Payload: sei},
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: nonIDR},
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: sei},
			},
			outcomes: [][][]byte{{picture}, nil, {sei, nonIDR}, nil},
			waiting:  []bool{false, true, false, true},
		},
		{
			name: "aggregated SEIs",
			packets: []*rtp.Packet{
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: picture},
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: []byte{0x18, 0, 3, 6, 5, 1, 0, 3, 6, 5, 2}},
			},
			outcomes: [][][]byte{{picture}, nil},
			waiting:  []bool{false, true},
		},
		{
			name: "timestamp boundary with marked picture",
			packets: []*rtp.Packet{
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: picture},
				{Header: rtp.Header{Timestamp: 100}, Payload: sei},
				{Header: rtp.Header{Marker: true, Timestamp: 200}, Payload: nonIDR},
				{Header: rtp.Header{Marker: true, Timestamp: 200}, Payload: sei},
				{Header: rtp.Header{Marker: true, Timestamp: 300}, Payload: nonIDR},
				{Header: rtp.Header{Marker: true, Timestamp: 300}, Payload: sei},
			},
			outcomes: [][][]byte{{picture}, nil, nil, {nonIDR, sei}, {nonIDR}, nil},
			waiting:  []bool{false, true, true, false, false, true},
		},
		{
			name: "timestamp boundary",
			packets: []*rtp.Packet{
				{Header: rtp.Header{Marker: true, Timestamp: 100}, Payload: picture},
				{Header: rtp.Header{Timestamp: 100}, Payload: sei},
				{Header: rtp.Header{Timestamp: 200}, Payload: nonIDR},
				{Header: rtp.Header{Marker: true, Timestamp: 200}, Payload: nonIDR},
				{Header: rtp.Header{Marker: true, Timestamp: 200}, Payload: sei},
			},
			outcomes: [][][]byte{{picture}, nil, nil, {nonIDR, nonIDR}, nil},
			waiting:  []bool{false, true, true, false, true},
		},
	}

	for _, ca := range tests {
		t.Run(ca.name, func(t *testing.T) {
			d := &rtph264.Decoder{PacketizationMode: 1}
			require.NoError(t, d.Init())

			for i, pkt := range ca.packets {
				au, err := d.Decode(pkt)
				if ca.waiting[i] {
					require.ErrorIs(t, err, rtph264.ErrMorePacketsNeeded)
				} else {
					require.NoError(t, err)
				}
				require.Equal(t, ca.outcomes[i], au)
			}
		})
	}
}

func serializePackets(packets []*rtp.Packet) ([]byte, error) {
	var buf []byte

	for _, pkt := range packets {
		buf2, err := pkt.Marshal()
		if err != nil {
			return nil, err
		}

		tmp := make([]byte, 4)
		binary.LittleEndian.PutUint32(tmp, uint32(len(buf2)))
		buf = append(buf, tmp...)
		buf = append(buf, buf2...)
	}

	return buf, nil
}

func unserializePackets(data []byte) ([]*rtp.Packet, error) {
	var packets []*rtp.Packet
	buf := data

	for {
		if len(buf) < 4 {
			return nil, errors.New("not enough bits")
		}

		size := binary.LittleEndian.Uint32(buf[:4])
		buf = buf[4:]

		if uint32(len(buf)) < size {
			return nil, errors.New("not enough bits")
		}

		var pkt rtp.Packet
		err := pkt.Unmarshal(buf[:size])
		if err != nil {
			return nil, err
		}

		packets = append(packets, &pkt)
		buf = buf[size:]

		if len(buf) == 0 {
			break
		}
	}

	return packets, nil
}

func FuzzDecoder(f *testing.F) {
	for _, ca := range cases {
		buf, err := serializePackets(ca.pkts)
		if err != nil {
			panic(err)
		}
		f.Add(buf)
	}

	for _, ca := range casesDecodeOnly {
		buf, err := serializePackets(ca.pkts)
		if err != nil {
			panic(err)
		}
		f.Add(buf)
	}

	buf, err := serializePackets([]*rtp.Packet{{
		Header:  rtp.Header{Marker: true},
		Payload: []byte{0x1c, 0xc0, 0x00, 0x00, 0x01},
	}})
	if err != nil {
		panic(err)
	}
	f.Add(buf)

	f.Fuzz(func(t *testing.T, buf []byte) {
		packets, err2 := unserializePackets(buf)
		if err2 != nil {
			t.Skip()
			return
		}

		d := &rtph264.Decoder{PacketizationMode: 1}
		err2 = d.Init()
		require.NoError(t, err2)

		for _, pkt := range packets {
			var au [][]byte
			au, err2 = d.Decode(pkt)
			if err2 != nil {
				continue
			}

			require.NotEmpty(t, au)

			for _, nalu := range au {
				require.NotEmpty(t, nalu)
			}

			e := &rtph264.Encoder{
				PacketizationMode:     1,
				SSRC:                  new(uint32(12321)),
				InitialSequenceNumber: new(uint16(45432)),
			}
			err2 = e.Init()
			require.NoError(t, err2)

			e.Encode(au) //nolint:errcheck
		}
	})
}
