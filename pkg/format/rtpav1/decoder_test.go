package rtpav1

import (
	"errors"
	"testing"

	"github.com/bluenviron/mediacommon/v2/pkg/codecs/av1"
	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			d := &Decoder{}
			err := d.Init()
			require.NoError(t, err)

			var obus [][]byte

			for _, pkt := range ca.pkts {
				addOBUs, err := d.Decode(pkt)
				if errors.Is(err, ErrMorePacketsNeeded) {
					continue
				}

				require.NoError(t, err)
				obus = append(obus, addOBUs...)
			}

			require.Equal(t, ca.obus, obus)
		})
	}
}

func TestDecoderErrorOBUCount(t *testing.T) {
	d := &Decoder{}
	err := d.Init()
	require.NoError(t, err)

	for i := 0; i <= av1.MaxOBUsPerTemporalUnit; i++ {
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

	require.EqualError(t, err, "OBU count (11) exceeds maximum allowed (10)")
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
		Payload: []byte{ //nolint:dupl
			0x40, 0xb1, 0x0b, 0x30, 0x30, 0xca, 0x08, 0x12,
			0xfd, 0xfd, 0x78, 0x89, 0xe5, 0x3e, 0xa3, 0x80,
			0x08, 0x20, 0x82, 0x08, 0x54, 0x50, 0x00, 0x30,
			0x00, 0x00, 0x66, 0x00, 0xeb, 0xcd, 0xc5, 0x72,
			0x25, 0x5a, 0x0a, 0xc3, 0x4a, 0x43, 0x15, 0x59,
			0x13, 0x5f, 0xb9, 0x39, 0x19, 0xf1, 0xca, 0x26,
			0xc7, 0x59, 0xa7, 0x66, 0xe4, 0xa5, 0xff, 0x3a,
			0xba, 0x3f, 0x62, 0xfc, 0x58, 0x07, 0xd0, 0x18,
			0xd6, 0xb2, 0x90, 0x72, 0xf4, 0x57, 0xbd, 0x5d,
			0x72, 0x92, 0x8c, 0x71, 0x48, 0x17, 0x89, 0x65,
			0xa9, 0x9e, 0xa2, 0x81, 0x9a, 0x83, 0x66, 0x11,
			0xd7, 0xe7, 0x41, 0x35, 0xb7, 0x8a, 0xb3, 0xd0,
			0x90, 0x54, 0x2a, 0x48, 0xea, 0x65, 0xd1, 0xb0,
			0x83, 0xda, 0xbb, 0x61, 0xff, 0xc4, 0xd0, 0x06,
			0x78, 0x35, 0xc2, 0xf7, 0x15, 0x16, 0xe5, 0xc3,
			0xf0, 0x9f, 0x7f, 0x1a, 0xd6, 0xad, 0xd4, 0x41,
			0xa4, 0xf4, 0x40, 0x41, 0x00, 0x78, 0x5c, 0x14,
			0x08, 0x57, 0x3d, 0x96, 0xc2, 0x0b, 0xb5, 0x03,
			0xce, 0x7d, 0xe6, 0x50, 0xd1, 0xbc, 0x3e, 0x1e,
			0x84, 0x62, 0x1a, 0xd1, 0xfe, 0x52, 0x9e, 0x79,
			0x26, 0x51, 0xc8, 0x14, 0x54, 0xf2, 0x77, 0xe6,
			0x25, 0x0f, 0x5f, 0xec, 0x66, 0x33, 0xdb, 0xd0,
			0x9f, 0x5a, 0xf9, 0xbc, 0xc4, 0x9b, 0x31, 0x8c,
			0x6f, 0xe6, 0x2a, 0x50, 0x65, 0x1d, 0x45, 0xaf,
			0x19, 0xd4, 0xde, 0xf8, 0x33, 0x43, 0x48, 0x57,
			0x00, 0x80, 0x5b, 0xaa, 0x3e, 0x1b, 0xf7, 0x21,
			0xc1, 0x56, 0x5e, 0xfb, 0xdb, 0x43, 0xa8, 0x7b,
			0xde, 0x82, 0xc1, 0x45, 0x5a, 0x36, 0x04, 0x20,
			0xa8, 0xa3, 0x0b, 0xa9, 0xa8, 0x6d, 0xe1, 0xfd,
			0x13, 0x0d, 0x38, 0xef, 0x00, 0xd5, 0x00, 0xe5,
			0x2d, 0x69, 0x67, 0x17, 0x59, 0xd9, 0xda, 0x61,
			0x95, 0xac, 0x55, 0x3a, 0xc5, 0x77, 0x1e, 0xbe,
			0xa5, 0x58, 0x7a, 0xbb, 0x09, 0xbd, 0x46, 0xab,
			0x82, 0x60, 0x13, 0x8f, 0x8b, 0xfb, 0x29, 0xe0,
			0x7f, 0xa5, 0xca, 0x25, 0xfa, 0x77, 0x07, 0x88,
			0x51, 0xdb, 0xc2, 0x99, 0x5f, 0xad, 0xe1, 0x45,
			0x28, 0x0e, 0x00, 0x86, 0x79, 0x60, 0xe2, 0x36,
			0x9a, 0x41, 0x21, 0x01, 0xdf, 0x3d, 0x0c, 0x31,
			0x37, 0xa4, 0x0b, 0x00, 0xe6, 0x43, 0xfb, 0x28,
			0x05, 0x2c, 0xff, 0x40, 0x1c, 0x23, 0x6d, 0x4f,
			0x58, 0x00, 0xe1, 0xc4, 0xb2, 0x05, 0x3a, 0x03,
			0xa4, 0xde, 0xf4, 0x13, 0x5e, 0x8c, 0x45, 0xc2,
			0x3b, 0xf0, 0x2c, 0x00, 0x28, 0x0b, 0xe2, 0x7d,
			0xa4, 0x27, 0x78, 0xf0, 0xfa, 0x3f, 0x0a, 0xa9,
			0xac, 0x20, 0xa0, 0xe2, 0x33, 0xe8, 0xc6, 0xa7,
			0x69, 0x6f, 0x4c, 0xec, 0x67, 0x06, 0x79, 0x27,
			0x1d, 0x69, 0xe3, 0xa6, 0xad, 0x6e, 0x63, 0x97,
			0xd2, 0xe3, 0xaa, 0xd4, 0xc5, 0x6d, 0xaa, 0x29,
			0xfa, 0xc3, 0x4b, 0x23, 0xec, 0xe7, 0x5f, 0xdd,
			0xf8, 0x45, 0x55, 0x62, 0xec, 0x3c, 0xe8, 0x73,
			0xf0, 0x63, 0x04, 0x6f, 0x88, 0xa7, 0x4d, 0x7c,
			0x28, 0xef, 0x20, 0x0d, 0x00, 0xf9, 0xba, 0xe5,
			0x09, 0x5d, 0xc2, 0x8d, 0x7d, 0x18, 0x2c, 0x2f,
			0xff, 0x93, 0x88, 0x2d, 0x00, 0xf9, 0xaf, 0x22,
			0x68, 0x70, 0x72, 0xe4, 0x36, 0x22, 0x15, 0x5f,
			0x7a, 0xef, 0xd9, 0x03, 0x25, 0xf5, 0x17, 0x68,
			0x30, 0xee, 0x2b, 0xed, 0xe8, 0x0c, 0x1a, 0x32,
			0x72, 0x62, 0xd4, 0x74, 0x69, 0xf1, 0x9a, 0xcf,
			0x9b, 0x52, 0xfc, 0x56, 0xe3, 0xda, 0x73, 0x09,
			0x33, 0x95, 0x08, 0x77, 0x00, 0x09, 0x17, 0x4d,
			0xb7, 0xc4, 0x32, 0xcd, 0xf9, 0x33, 0x7f, 0x78,
			0x44, 0x33, 0xf3, 0x94, 0x5d, 0x74, 0x14, 0x53,
			0x3e, 0xc1, 0xa6, 0x62, 0x3a, 0x94, 0x41, 0xc9,
			0x4d, 0x05, 0x87, 0x7d, 0x92, 0xd9, 0x12, 0xb4,
			0x9a, 0x1f, 0x43, 0xf4, 0x54, 0x0c, 0xb4, 0x6c,
			0xd0, 0x16, 0xd7, 0x46, 0xe2, 0x5e, 0xdc, 0x56,
			0xff, 0x42, 0x72, 0xe4, 0x97, 0x8f, 0xdb, 0x2f,
			0x27, 0xbd, 0x4c, 0xa3, 0x4e, 0x43, 0x4a, 0x0b,
			0x46, 0x45, 0x36, 0x80, 0x0d, 0xd4, 0x87, 0xc7,
			0x1a, 0xf8, 0x5f, 0x9c, 0xf4, 0x10, 0x82, 0xa8,
			0x28, 0xa7, 0xd1, 0xbf, 0xc1, 0x61, 0x5a, 0x73,
			0xbd, 0xe9, 0x44, 0x66, 0x5a, 0x2b, 0x14, 0x4c,
			0xa5, 0x5d, 0x3d, 0x38, 0xfb, 0xd4, 0xa8, 0x24,
			0xbe, 0xe4, 0x99, 0xa1, 0x98, 0x90, 0x72, 0x67,
			0x1f, 0x3b, 0x96, 0xae, 0x60, 0x7a, 0x00, 0xf2,
			0x11, 0x13, 0x9f, 0xc4, 0xb5, 0x63, 0xd7, 0x63,
			0xb6, 0x64, 0xe3, 0xbb, 0xf1, 0x96, 0x9e, 0x77,
			0xab, 0xc4, 0xa8, 0x39, 0xd5, 0x22, 0xe9, 0xee,
			0xad, 0x1f, 0x91, 0xff, 0x56, 0x3e, 0x68, 0x89,
			0x63, 0x11, 0xc8, 0x98, 0xa1, 0x7e, 0xd9, 0x37,
			0xb2, 0x94, 0x23, 0xfb, 0x67, 0xb7, 0xa0, 0xaa,
			0x0f, 0xe7, 0xb6, 0xe1, 0xb8, 0xc3, 0x73, 0xe3,
			0xae, 0x07, 0xe6, 0xf7, 0xc8, 0x35, 0x03, 0xc5,
			0x7a, 0xfc, 0x48, 0xd4, 0xe2, 0x61, 0x7a, 0x22,
			0xdd, 0x78, 0x4b, 0xef, 0xa4, 0xde, 0x27, 0x62,
			0xdd, 0xa1, 0x76, 0xe5, 0xeb, 0x83, 0xa3, 0x18,
			0xde, 0xe9, 0x99, 0x37, 0x66, 0x34, 0x20, 0xaf,
			0x67, 0x2b, 0xdd, 0xa4, 0x69, 0xeb, 0x7d, 0x71,
			0x0f, 0x24, 0xef, 0x88, 0x49, 0x8c, 0xe5, 0x33,
			0xfa, 0xf3, 0x3e, 0xff, 0x46, 0x10, 0x5d, 0x25,
			0x97, 0x7f, 0x8e, 0x00, 0xe2, 0x08, 0x0e, 0x39,
			0x65, 0x4c, 0xa5, 0x1b, 0x5f, 0x5a, 0xa8, 0xab,
			0xb1, 0x4a, 0x55, 0xf0, 0x41, 0x9e, 0xf5, 0xcd,
			0x2c, 0x34, 0x56, 0x11, 0xda, 0x96, 0x68, 0xe4,
			0x39, 0x8d, 0xfe, 0x40, 0xb0, 0x0b, 0x05, 0x5d,
			0xf4, 0xba, 0x27, 0x1a, 0x3b, 0xed, 0xee, 0x88,
			0x07, 0x59, 0x40, 0xf6, 0x6f, 0x44, 0x1a, 0x5f,
			0x3d, 0x42, 0xb2, 0xa7, 0x5d, 0x07, 0xd5, 0x7e,
			0x95, 0x36, 0xc9, 0xa3, 0xd3, 0x45, 0x56, 0x70,
			0x92, 0x79, 0xc9, 0xd7, 0xfd, 0x88, 0xa4, 0x89,
			0x9a, 0x94, 0xbd, 0x9d, 0x49, 0xec, 0x32, 0xc4,
			0xee, 0x87, 0xd7, 0x0a, 0xf8, 0xab, 0x23, 0x70,
			0x3b, 0xbd, 0xd6, 0x34, 0x86, 0x3a, 0x05, 0x68,
			0x95, 0x31, 0x81, 0x2d, 0x6d, 0xde, 0x54, 0xfe,
			0x0b, 0xa9, 0x72, 0x3c, 0x78, 0xb7, 0xf5, 0x84,
			0x67, 0x39, 0x4f, 0x2d, 0xbb, 0xcb, 0x22, 0x2d,
			0xed, 0xf2, 0xc1, 0x56, 0xf8, 0x11, 0xf9, 0xee,
			0x81, 0xd8, 0x64, 0x07, 0xbf, 0xb4, 0x69, 0xdb,
			0xda, 0x0c, 0xf5, 0x20, 0x00, 0xf8, 0x26, 0x7b,
			0x31, 0x54, 0x09, 0x24, 0xbf, 0x7c, 0xf5, 0xad,
			0x76, 0xbb, 0xcc, 0x89, 0xee, 0x5b, 0x2a, 0x23,
			0x12, 0x1c, 0x69, 0x50, 0xdd, 0x03, 0x0d, 0xe4,
			0xca, 0xb8, 0x59, 0x7c, 0x91, 0xa0, 0x82, 0x00,
			0x01, 0x9e, 0x1e, 0x4d, 0x87, 0xf7, 0xab, 0x3d,
			0x06, 0x6c, 0x95, 0x90, 0x63, 0x57, 0x0f, 0x44,
			0x4c, 0x49, 0x21, 0x2e, 0x85, 0x47, 0x61, 0x02,
			0x67, 0x57, 0xdc, 0xde, 0x67, 0x92, 0x11, 0x34,
			0x85, 0x05, 0x9e, 0x25, 0xe4, 0x1b, 0x8e, 0xaa,
			0xca, 0xd9, 0x4c, 0x2c, 0x84, 0x30, 0xf1, 0xdc,
			0x4a, 0x9b, 0x02, 0xdd, 0x69, 0x5e, 0xcd, 0x31,
			0x08, 0x27, 0xd1, 0x83, 0xcd, 0x03, 0x4d, 0xd9,
			0xf0, 0x0a, 0x69, 0x8e, 0x8b, 0x88, 0x14, 0x98,
			0x37, 0x8c, 0xad, 0x93, 0x70, 0x04, 0xf8, 0x88,
			0xa7, 0x67, 0xf0, 0xfe, 0xa9, 0x49, 0x46, 0xbe,
			0xab, 0x14, 0x8c, 0x56, 0xea, 0x17, 0x52, 0x1f,
			0x35, 0xfa, 0xd2, 0xd4, 0xa4, 0xf6, 0xb6, 0x72,
			0x25, 0x03, 0xbf, 0xc1, 0x85, 0xe1, 0x3e, 0x3a,
			0x19, 0xcb, 0xe3, 0x7e, 0x15, 0x65, 0xd1, 0x33,
			0x40, 0x2a, 0xd8, 0x3f, 0x2c, 0x86, 0x12, 0x65,
			0xe2, 0x60, 0x80, 0x1b, 0x01, 0x0b, 0xd2, 0xcd,
			0x3b, 0x24, 0x56, 0x73, 0xcd, 0xa9, 0x34, 0xd1,
			0x36, 0xd3, 0x26, 0x6f, 0x49, 0x58, 0xcf, 0xb7,
			0x39, 0x92, 0xa8, 0x15, 0x47, 0x0f, 0x92, 0x68,
			0x49, 0xa2, 0x91, 0xe0, 0x55, 0xc8, 0xd9, 0xf1,
			0x84, 0xc4, 0x41, 0x67, 0xd9, 0x24, 0xf3, 0x0f,
			0xdc, 0x3d, 0x6d, 0x79, 0x4d, 0x0f, 0x35, 0x8b,
			0xdf, 0x6d, 0xcd, 0x66, 0x7e, 0x82, 0x09, 0xa3,
			0x72, 0x1d, 0x3d, 0x4b, 0x0f, 0xad, 0x7c, 0xbe,
			0xdd, 0x96, 0x17, 0x62, 0x26, 0x72, 0xe6, 0x1d,
			0xb9, 0xf6, 0x9a, 0x71, 0x52, 0x5a, 0x76, 0xbb,
			0x4e, 0x11, 0x78, 0xbe, 0x05, 0xe5, 0xf6, 0xaf,
			0xf2, 0x66, 0xd3, 0xe5, 0xdb, 0xfc, 0xd8, 0x96,
			0x64, 0x8b, 0x7a, 0xec, 0x9f, 0x3e, 0x0d, 0x7c,
			0xd5, 0x20, 0xdf, 0x50, 0x52, 0x57, 0x93, 0xa0,
			0x03, 0x5a, 0xed, 0x5c, 0xb4, 0xfa, 0x15, 0xf0,
			0xda, 0xc4, 0x94, 0xbf, 0xcb, 0x25, 0xc7, 0xda,
			0x59, 0x5d, 0xb8, 0x00, 0x00, 0x00, 0x56, 0x76,
			0xe4, 0x62, 0x1c, 0x71, 0x70, 0xf6, 0x7f, 0x20,
			0x83, 0xee, 0x17, 0x23, 0x56, 0x8c, 0x20, 0x9c,
			0x65, 0x25, 0xe9, 0x38, 0xe2, 0xd2, 0x66, 0xae,
			0xa1, 0x47, 0x16, 0xba, 0x5e, 0x87, 0xd4, 0x8b,
			0x7e, 0x80, 0x9f, 0x83, 0x4b, 0x27, 0x46, 0x97,
			0x72, 0x2b, 0x88, 0xd1, 0xf0, 0x92, 0xd4, 0x2a,
			0x7e, 0x48, 0x63, 0x95, 0x33, 0x13, 0x07, 0xf8,
			0x79, 0x3a, 0x6b, 0x67, 0x1c, 0x6f, 0x38, 0x80,
			0x3c, 0xc6, 0xfa, 0xb8, 0x4e, 0xd2, 0x01, 0xaf,
			0x79, 0x37, 0x60, 0xd6, 0x4d, 0x55, 0xcd, 0x1d,
			0xf5, 0x15, 0x0f, 0xb7, 0x8c, 0xa5, 0xec, 0x25,
			0xe4, 0x45, 0xec, 0xe6, 0xf9, 0xc2, 0x35, 0xea,
			0xc6, 0x30, 0xa2, 0x1f, 0x21, 0x13, 0x1f, 0x97,
			0x95, 0xee, 0x69, 0x63, 0xcf, 0x4b, 0xb2, 0xc3,
			0xc0, 0x23, 0xc9, 0x75, 0x2b, 0xd6, 0xac, 0x3c,
			0xcb, 0x38, 0x73, 0xa0, 0x5a, 0xa5, 0x7d, 0x3d,
			0x23, 0xb1, 0x4c, 0xc9, 0x1b, 0x12, 0xc4, 0x83,
			0xe1, 0x62, 0x61, 0xf3, 0x8c, 0x6a, 0x45, 0x95,
			0xee, 0x74, 0x00, 0x2f, 0x84, 0x77, 0xff, 0x16,
			0x22, 0x02, 0xfe, 0x4b, 0xd2, 0x57, 0x65, 0xee,
			0x49, 0xcd, 0x7b, 0xb5, 0xdb, 0x8e, 0x5a, 0xcb,
			0x70, 0x7f, 0x78, 0xbc, 0x35, 0x34, 0x3e, 0xcd,
			0xf6, 0x24, 0x78, 0x8f, 0xe8, 0x9e, 0x5c, 0x9c,
			0x22, 0x4a, 0xf0, 0xbc, 0x89, 0x64, 0x93, 0xcf,
			0x20, 0xc8, 0xfd, 0x27, 0xe2, 0x36, 0x86, 0x00,
			0x61, 0xe8, 0xe2, 0x9c, 0xad, 0x55, 0x98, 0x77,
			0x2f, 0xfa, 0x28, 0xb4, 0xa7, 0xda, 0xa4, 0x14,
			0x06, 0x8c, 0x7a, 0xd5, 0xdf, 0x0d, 0x32, 0x56,
			0xcc, 0xd3, 0x70, 0x71, 0x70, 0xd5, 0xb6, 0x41,
			0x0f, 0x45, 0xb2, 0xf0, 0x24, 0xa5, 0x23, 0x13,
			0x70, 0x36, 0x07, 0xcd, 0xc6, 0x75, 0xd8, 0x62,
			0x8c, 0x7f, 0xa7, 0xb9, 0x76, 0x16, 0xb3, 0x9a,
			0x7e, 0x20, 0x27, 0x27, 0x2d, 0x33, 0x60, 0x70,
			0xa5, 0xef, 0x5c, 0x9c, 0x6e, 0x5f, 0x2f, 0x20,
			0x78, 0x58, 0x59, 0x92, 0x15, 0xfe, 0x13, 0x07,
			0x87, 0xc7, 0x3e, 0x21,
		},
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
		Payload: []byte{
			0x80, 0xbb, 0x01, 0x04, 0x24, 0x33, 0x60, 0x15,
			0x91, 0xfb, 0xfe, 0xe8, 0xf8, 0xb9, 0xa6, 0x9a,
			0xbf, 0x65, 0xf3, 0x37, 0x40, 0xb8, 0x52, 0x69,
			0x3f, 0x65, 0x27, 0x36, 0x01, 0x92, 0xe8, 0xef,
			0x51, 0x3e, 0xcd, 0xf5, 0xcc, 0x4f, 0x8b, 0x4f,
			0x7b, 0x38, 0x3e, 0x43, 0x4d, 0xc0, 0xa4, 0xc6,
			0x84, 0x79, 0x8e, 0xf4, 0x4a, 0xe1, 0xa9, 0x2b,
			0xd5, 0x34, 0x00, 0x30, 0x94, 0x6a, 0x91, 0xa2,
			0xd5, 0xc0, 0xbf, 0xb5, 0x66, 0x59, 0x38, 0xc2,
			0xa4, 0x75, 0x96, 0x5b, 0x7a, 0xac, 0x9c, 0x05,
			0x67, 0x99, 0x2e, 0xba, 0x21, 0x1e, 0xac, 0x50,
			0x01, 0x68, 0x71, 0xac, 0x68, 0x6d, 0xc1, 0xf2,
			0x2c, 0x51, 0x1e, 0xb6, 0x64, 0x97, 0x3d, 0x9b,
			0x2b, 0x89, 0x66, 0xbd, 0x50, 0x4f, 0x5c, 0xec,
			0x7c, 0xc8, 0x51, 0x61, 0x74, 0xbd, 0x86, 0xb4,
			0x6e, 0x71, 0x59, 0x24, 0x59, 0xaf, 0x66, 0x54,
			0x82, 0xdb, 0xe2, 0x11, 0x6f, 0x87, 0x92, 0xf3,
			0x63, 0xd4, 0x4b, 0xd6, 0x26, 0x42, 0xf1, 0x2a,
			0x66, 0x63, 0x79, 0x5e, 0x7e, 0xfb, 0x3f, 0x85,
			0xa7, 0x24, 0xac, 0x53, 0xe2, 0x57, 0x56, 0xdd,
			0xef, 0x7a, 0xdf, 0xb6, 0x27, 0xaf, 0x25, 0x6a,
			0x89, 0x4a, 0x4c, 0xc3, 0xe6, 0x2e, 0xbe, 0x2b,
			0x38, 0x07, 0xe0, 0x96, 0x98, 0xaf, 0x85, 0xf4,
			0x86, 0x7d, 0xf4, 0xee, 0xe5, 0x78,
		},
	})
	require.EqualError(t, err, "discarding frame since a RTP packet is missing")
}

func FuzzDecoder(f *testing.F) {
	f.Fuzz(func(t *testing.T, a []byte, am bool, b []byte, bm bool) {
		d := &Decoder{}
		err := d.Init()
		require.NoError(t, err)

		tu, err := d.Decode(&rtp.Packet{
			Header: rtp.Header{
				Marker:         am,
				SequenceNumber: 17645,
			},
			Payload: a,
		})

		if errors.Is(err, ErrMorePacketsNeeded) {
			tu, err = d.Decode(&rtp.Packet{
				Header: rtp.Header{
					Marker:         bm,
					SequenceNumber: 17646,
				},
				Payload: b,
			})
		}

		if err == nil {
			if len(tu) == 0 {
				t.Errorf("should not happen")
			}

			for _, nalu := range tu {
				if len(nalu) == 0 {
					t.Errorf("should not happen")
				}
			}
		}
	})
}
