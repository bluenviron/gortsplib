package rtpmpeg1audio

import (
	"testing"

	"github.com/pion/rtp"
	"github.com/stretchr/testify/require"
)

func uint16Ptr(v uint16) *uint16 {
	return &v
}

func uint32Ptr(v uint32) *uint32 {
	return &v
}

var cases = []struct {
	name   string
	frames [][]byte
	pkts   []*rtp.Packet
}{
	{
		"single",
		[][]byte{{
			0xff, 0xfb, 0x14, 0x64, 0x00, 0x0f, 0xf0, 0x00,
			0x00, 0x69, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00,
			0x0d, 0x20, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01,
			0xa4, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x34,
			0x80, 0x00, 0x00, 0x04, 0x4c, 0x41, 0x4d, 0x45,
			0x33, 0x2e, 0x31, 0x30, 0x30, 0x55, 0x55, 0x55,
			0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
			0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
			0x55, 0xc0, 0x65, 0xf4, 0xa0, 0x31, 0x8f, 0xce,
			0x8d, 0x46, 0xfc, 0x8c, 0x73, 0xb9, 0x34, 0x3e,
			0xb5, 0x03, 0x39, 0xc0, 0x04, 0x01, 0x98, 0x44,
			0x38, 0xe0, 0x98, 0x10, 0x9b, 0xa8, 0x0f, 0xa8,
		}},
		[]*rtp.Packet{{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    14,
				SequenceNumber: 17645,
				Timestamp:      2289526357,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{
				0x00, 0x00, 0x00, 0x00,
				0xff, 0xfb, 0x14, 0x64, 0x00, 0x0f, 0xf0, 0x00,
				0x00, 0x69, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00,
				0x0d, 0x20, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01,
				0xa4, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x34,
				0x80, 0x00, 0x00, 0x04, 0x4c, 0x41, 0x4d, 0x45,
				0x33, 0x2e, 0x31, 0x30, 0x30, 0x55, 0x55, 0x55,
				0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
				0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
				0x55, 0xc0, 0x65, 0xf4, 0xa0, 0x31, 0x8f, 0xce,
				0x8d, 0x46, 0xfc, 0x8c, 0x73, 0xb9, 0x34, 0x3e,
				0xb5, 0x03, 0x39, 0xc0, 0x04, 0x01, 0x98, 0x44,
				0x38, 0xe0, 0x98, 0x10, 0x9b, 0xa8, 0x0f, 0xa8,
			},
		}},
	},
	{
		"aggregated",
		[][]byte{
			{
				0xff, 0xfb, 0x14, 0x64, 0x00, 0x0f, 0xf0, 0x00,
				0x00, 0x69, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00,
				0x0d, 0x20, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01,
				0xa4, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x34,
				0x80, 0x00, 0x00, 0x04, 0x4c, 0x41, 0x4d, 0x45,
				0x33, 0x2e, 0x31, 0x30, 0x30, 0x55, 0x55, 0x55,
				0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
				0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
				0x55, 0xc0, 0x65, 0xf4, 0xa0, 0x31, 0x8f, 0xce,
				0x8d, 0x46, 0xfc, 0x8c, 0x73, 0xb9, 0x34, 0x3e,
				0xb5, 0x03, 0x39, 0xc0, 0x04, 0x01, 0x98, 0x44,
				0x38, 0xe0, 0x98, 0x10, 0x9b, 0xa8, 0x0f, 0xa8,
			},
			{
				0xff, 0xfb, 0x14, 0x64, 0x1e, 0x0f, 0xf0, 0x00,
				0x00, 0x69, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00,
				0x0d, 0x20, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01,
				0xa4, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x34,
				0x80, 0x00, 0x00, 0x04, 0xe6, 0x50, 0x10, 0x01,
				0xca, 0x13, 0x94, 0x27, 0x4a, 0x4a, 0x64, 0xce,
				0x07, 0xc2, 0x2f, 0x59, 0xc0, 0x19, 0x04, 0x05,
				0xdf, 0xe7, 0xce, 0x65, 0x24, 0xed, 0xa4, 0xe3,
				0xff, 0xc9, 0x00, 0x00, 0x05, 0x5f, 0x4a, 0x04,
				0x0e, 0xc4, 0x24, 0xfd, 0x5e, 0x4a, 0x35, 0x72,
				0x21, 0x27, 0x31, 0x08, 0x47, 0x18, 0x00, 0x06,
				0xc4, 0x02, 0x72, 0x81, 0x89, 0xc3, 0xe4, 0x0a,
			},
		},
		[]*rtp.Packet{{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:    14,
				SequenceNumber: 17645,
				Timestamp:      2289526357,
				SSRC:           0x9dbb7812,
			},
			Payload: []byte{
				0x00, 0x00, 0x00, 0x00,
				0xff, 0xfb, 0x14, 0x64, 0x00, 0x0f, 0xf0, 0x00,
				0x00, 0x69, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00,
				0x0d, 0x20, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01,
				0xa4, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x34,
				0x80, 0x00, 0x00, 0x04, 0x4c, 0x41, 0x4d, 0x45,
				0x33, 0x2e, 0x31, 0x30, 0x30, 0x55, 0x55, 0x55,
				0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
				0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55,
				0x55, 0xc0, 0x65, 0xf4, 0xa0, 0x31, 0x8f, 0xce,
				0x8d, 0x46, 0xfc, 0x8c, 0x73, 0xb9, 0x34, 0x3e,
				0xb5, 0x03, 0x39, 0xc0, 0x04, 0x01, 0x98, 0x44,
				0x38, 0xe0, 0x98, 0x10, 0x9b, 0xa8, 0x0f, 0xa8,
				0xff, 0xfb, 0x14, 0x64, 0x1e, 0x0f, 0xf0, 0x00,
				0x00, 0x69, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00,
				0x0d, 0x20, 0x00, 0x00, 0x01, 0x00, 0x00, 0x01,
				0xa4, 0x00, 0x00, 0x00, 0x20, 0x00, 0x00, 0x34,
				0x80, 0x00, 0x00, 0x04, 0xe6, 0x50, 0x10, 0x01,
				0xca, 0x13, 0x94, 0x27, 0x4a, 0x4a, 0x64, 0xce,
				0x07, 0xc2, 0x2f, 0x59, 0xc0, 0x19, 0x04, 0x05,
				0xdf, 0xe7, 0xce, 0x65, 0x24, 0xed, 0xa4, 0xe3,
				0xff, 0xc9, 0x00, 0x00, 0x05, 0x5f, 0x4a, 0x04,
				0x0e, 0xc4, 0x24, 0xfd, 0x5e, 0x4a, 0x35, 0x72,
				0x21, 0x27, 0x31, 0x08, 0x47, 0x18, 0x00, 0x06,
				0xc4, 0x02, 0x72, 0x81, 0x89, 0xc3, 0xe4, 0x0a,
			},
		}},
	},
	{
		"fragmented",
		[][]byte{{
			0xff, 0xfd, 0xc8, 0x00, 0x33, 0x33, 0x33, 0x66,
			0x66, 0x66, 0x66, 0x66, 0x66, 0x66, 0x65, 0xb6,
			0xdb, 0x64, 0x92, 0x49, 0x24, 0x6d, 0xb6, 0xdb,
			0xfa, 0xaa, 0x55, 0x0a, 0xaa, 0xaa, 0xaa, 0xaa,
			0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
			0xaa, 0xaa, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
			0xff, 0xff, 0xff, 0xff, 0xff, 0xf7, 0x77, 0x77,
			0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0xbd,
			0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
			0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
			0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
			0xbd, 0xef, 0x7b, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
			0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xad, 0x8b, 0x62,
			0xd8, 0xb6, 0x2d, 0x8b, 0x62, 0xd8, 0xb6, 0x2d,
			0x8b, 0x63, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb,
			0x6d, 0xb6, 0xdb, 0xff, 0xfb, 0xff, 0xfb, 0xff,
			0xfb, 0xff, 0xfb, 0xff, 0xfb, 0xff, 0xfb, 0xe7,
			0xcf, 0x9f, 0x3e, 0x7c, 0xd6, 0xb5, 0xae, 0xee,
			0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xef,
			0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
			0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
			0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
			0xef, 0x7b, 0xde, 0xf7, 0x77, 0x77, 0x77, 0x77,
			0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x5b, 0x16,
			0xc5, 0xb1, 0x6c, 0x5b, 0x16, 0xc5, 0xb1, 0x6c,
			0x5b, 0x16, 0xc6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d,
			0xb6, 0xdb, 0x6d, 0xb7, 0xff, 0xf7, 0xff, 0xf7,
			0xff, 0xf7, 0xff, 0xf7, 0xff, 0xf7, 0xff, 0xf7,
			0xcf, 0x9f, 0x3e, 0x7c, 0xf9, 0xad, 0x6b, 0x5d,
			0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd,
			0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
			0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
			0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
			0x7b, 0xde, 0xf7, 0xbd, 0xee, 0xee, 0xee, 0xee,
			0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xb6,
			0x2d, 0x8b, 0x62, 0xd8, 0xb6, 0x2d, 0x8b, 0x62,
			0xd8, 0xb6, 0x2d, 0x8d, 0xb6, 0xdb, 0x6d, 0xb6,
			0xdb, 0x6d, 0xb6, 0xdb, 0x6f, 0xff, 0xef, 0xff,
			0xef, 0xff, 0xef, 0xff, 0xef, 0xff, 0xef, 0xff,
			0xef, 0x9f, 0x3e, 0x7c, 0xf9, 0xf3, 0x5a, 0xd6,
			0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
			0xbb, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
			0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
			0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
			0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xdd, 0xdd, 0xdd,
			0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd,
			0x6c, 0x5b, 0x16, 0xc5, 0xb1, 0x6c, 0x5b, 0x16,
			0xc5, 0xb1, 0x6c, 0x5b, 0x1b, 0x6d, 0xb6, 0xdb,
			0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdf, 0xff, 0xdf,
			0xff, 0xdf, 0xff, 0xdf, 0xff, 0xdf, 0xff, 0xdf,
			0xff, 0xdf, 0x3e, 0x7c, 0xf9, 0xf3, 0xe6, 0xb5,
			0xad, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77,
			0x77, 0x77, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
			0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
			0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
			0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbb, 0xbb,
			0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
			0xba, 0xd8, 0xb6, 0x2d, 0x8b, 0x62, 0xd8, 0xb6,
			0x2d, 0x8b, 0x62, 0xd8, 0xb6, 0x36, 0xdb, 0x6d,
			0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xbf, 0xff,
			0xbf, 0xff, 0xbf, 0xff, 0xbf, 0xff, 0xbf, 0xff,
			0xbf, 0xff, 0xbe, 0x7c, 0xf9, 0xf3, 0xe7, 0xcd,
			0x6b, 0x5a, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
			0xee, 0xee, 0xee, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
			0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
			0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
			0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x77,
			0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77,
			0x77, 0x75, 0xb1, 0x6c, 0x5b, 0x16, 0xc5, 0xb1,
			0x6c, 0x5b, 0x16, 0xc5, 0xb1, 0x6c, 0x6d, 0xb6,
			0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x7f,
			0xff, 0x7f, 0xff, 0x7f, 0xff, 0x7f, 0xff, 0x7f,
			0xff, 0x7f, 0xff, 0x7c, 0xf9, 0xf3, 0xe7, 0xcf,
			0x9a, 0xd6, 0xb5, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd,
			0xdd, 0xdd, 0xdd, 0xdd, 0xef, 0x7b, 0xde, 0xf7,
			0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
			0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
			0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
			0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
			0xee, 0xee, 0xeb, 0x62, 0xd8, 0xb6, 0x2d, 0x8b,
			0x62, 0xd8, 0xb6, 0x2d, 0x8b, 0x62, 0xd8, 0xdb,
			0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6,
			0xff, 0xfe, 0xff, 0xfe, 0xff, 0xfe, 0xff, 0xfe,
			0xff, 0xfe, 0xff, 0xfe, 0xf9, 0xf3, 0xe7, 0xcf,
			0x9f, 0x35, 0xad, 0x6b, 0xbb, 0xbb, 0xbb, 0xbb,
			0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xde, 0xf7, 0xbd,
			0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
			0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
			0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
			0xbd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd,
			0xdd, 0xdd, 0xdd, 0xd6, 0xc5, 0xb1, 0x6c, 0x5b,
			0x16, 0xc5, 0xb1, 0x6c, 0x5b, 0x16, 0xc5, 0xb1,
			0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb,
			0x6d, 0xff, 0xfd, 0xff, 0xfd, 0xff, 0xfd, 0xff,
			0xfd, 0xff, 0xfd, 0xff, 0xfd, 0xf3, 0xe7, 0xcf,
			0x9f, 0x3e, 0x6b, 0x5a, 0xd7, 0x77, 0x77, 0x77,
			0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0xbd, 0xef,
			0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
			0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
			0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
			0xef, 0x7b, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
			0xbb, 0xbb, 0xbb, 0xbb, 0xad, 0x8b, 0x62, 0xd8,
			0xb6, 0x2d, 0x8b, 0x62, 0xd8, 0xb6, 0x2d, 0x8b,
			0x63, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d,
			0xb6, 0xdb, 0xff, 0xfb, 0xff, 0xfb, 0xff, 0xfb,
			0xff, 0xfb, 0xff, 0xfb, 0xff, 0xfb, 0xe7, 0xcf,
			0x9f, 0x3e, 0x7c, 0xd6, 0xb5, 0xae, 0xee, 0xee,
			0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xef, 0x7b,
			0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
			0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
			0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
			0x7b, 0xde, 0xf7, 0x77, 0x77, 0x77, 0x77, 0x77,
			0x77, 0x77, 0x77, 0x77, 0x77, 0x5b, 0x16, 0xc5,
			0xb1, 0x6c, 0x5b, 0x16, 0xc5, 0xb1, 0x6c, 0x5b,
			0x16, 0xc6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6,
			0xdb, 0x6d, 0xb7, 0xff, 0xf7, 0xff, 0xf7, 0xff,
			0xf7, 0xff, 0xf7, 0xff, 0xf7, 0xff, 0xf7, 0xcf,
			0x9f, 0x3e, 0x7c, 0xf9, 0xad, 0x6b, 0x5d, 0xdd,
			0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xde,
			0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
			0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
			0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
			0xde, 0xf7, 0xbd, 0xee, 0xee, 0xee, 0xee, 0xee,
			0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xb6, 0x2d,
			0x8b, 0x62, 0xd8, 0xb6, 0x2d, 0x8b, 0x62, 0xd8,
			0xb6, 0x2d, 0x8d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb,
			0x6d, 0xb6, 0xdb, 0x6f, 0xff, 0xef, 0xff, 0xef,
			0xff, 0xef, 0xff, 0xef, 0xff, 0xef, 0xff, 0xef,
			0x9f, 0x3e, 0x7c, 0xf9, 0xf3, 0x5a, 0xd6, 0xbb,
			0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
			0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
			0xde, 0xf7, 0xbd, 0xef,
			0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
			0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
			0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd,
			0xdd, 0xdd, 0xdd, 0x6c, 0x5b, 0x16, 0xc5, 0xb1,
			0x6c, 0x5b, 0x16, 0xc5, 0xb1, 0x6c, 0x5b, 0x1b,
			0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6,
			0xdf, 0xff, 0xdf, 0xff, 0xdf, 0xff, 0xdf, 0xff,
			0xdf, 0xff, 0xdf, 0xff, 0xdf, 0x3e, 0x7c, 0xf9,
			0xf3, 0xe6, 0xb5, 0xad,
		}},
		[]*rtp.Packet{
			{ //nolint:dupl
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    14,
					SequenceNumber: 17645,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{
					0x00, 0x00, 0x00, 0x00, 0xff, 0xfd, 0xc8, 0x00,
					0x33, 0x33, 0x33, 0x66, 0x66, 0x66, 0x66, 0x66,
					0x66, 0x66, 0x65, 0xb6, 0xdb, 0x64, 0x92, 0x49,
					0x24, 0x6d, 0xb6, 0xdb, 0xfa, 0xaa, 0x55, 0x0a,
					0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa,
					0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xaa, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
					0xff, 0xf7, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77,
					0x77, 0x77, 0x77, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
					0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
					0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
					0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xbb,
					0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
					0xbb, 0xad, 0x8b, 0x62, 0xd8, 0xb6, 0x2d, 0x8b,
					0x62, 0xd8, 0xb6, 0x2d, 0x8b, 0x63, 0x6d, 0xb6,
					0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0xff,
					0xfb, 0xff, 0xfb, 0xff, 0xfb, 0xff, 0xfb, 0xff,
					0xfb, 0xff, 0xfb, 0xe7, 0xcf, 0x9f, 0x3e, 0x7c,
					0xd6, 0xb5, 0xae, 0xee, 0xee, 0xee, 0xee, 0xee,
					0xee, 0xee, 0xee, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
					0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
					0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
					0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
					0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77,
					0x77, 0x77, 0x5b, 0x16, 0xc5, 0xb1, 0x6c, 0x5b,
					0x16, 0xc5, 0xb1, 0x6c, 0x5b, 0x16, 0xc6, 0xdb,
					0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb7,
					0xff, 0xf7, 0xff, 0xf7, 0xff, 0xf7, 0xff, 0xf7,
					0xff, 0xf7, 0xff, 0xf7, 0xcf, 0x9f, 0x3e, 0x7c,
					0xf9, 0xad, 0x6b, 0x5d, 0xdd, 0xdd, 0xdd, 0xdd,
					0xdd, 0xdd, 0xdd, 0xdd, 0xde, 0xf7, 0xbd, 0xef,
					0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
					0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
					0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
					0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
					0xee, 0xee, 0xee, 0xb6, 0x2d, 0x8b, 0x62, 0xd8,
					0xb6, 0x2d, 0x8b, 0x62, 0xd8, 0xb6, 0x2d, 0x8d,
					0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb,
					0x6f, 0xff, 0xef, 0xff, 0xef, 0xff, 0xef, 0xff,
					0xef, 0xff, 0xef, 0xff, 0xef, 0x9f, 0x3e, 0x7c,
					0xf9, 0xf3, 0x5a, 0xd6, 0xbb, 0xbb, 0xbb, 0xbb,
					0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbd, 0xef, 0x7b,
					0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
					0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
					0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
					0x7b, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd,
				},
			},
			{ //nolint:dupl
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    14,
					SequenceNumber: 17646,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{
					0x00, 0x00, 0x01, 0x8c, 0xdd, 0xdd, 0xdd, 0xdd,
					0x6c, 0x5b, 0x16, 0xc5, 0xb1, 0x6c, 0x5b, 0x16,
					0xc5, 0xb1, 0x6c, 0x5b, 0x1b, 0x6d, 0xb6, 0xdb,
					0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdf, 0xff, 0xdf,
					0xff, 0xdf, 0xff, 0xdf, 0xff, 0xdf, 0xff, 0xdf,
					0xff, 0xdf, 0x3e, 0x7c, 0xf9, 0xf3, 0xe6, 0xb5,
					0xad, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77,
					0x77, 0x77, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
					0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
					0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
					0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbb, 0xbb,
					0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
					0xba, 0xd8, 0xb6, 0x2d, 0x8b, 0x62, 0xd8, 0xb6,
					0x2d, 0x8b, 0x62, 0xd8, 0xb6, 0x36, 0xdb, 0x6d,
					0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xbf, 0xff,
					0xbf, 0xff, 0xbf, 0xff, 0xbf, 0xff, 0xbf, 0xff,
					0xbf, 0xff, 0xbe, 0x7c, 0xf9, 0xf3, 0xe7, 0xcd,
					0x6b, 0x5a, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
					0xee, 0xee, 0xee, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
					0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
					0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
					0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x77,
					0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77,
					0x77, 0x75, 0xb1, 0x6c, 0x5b, 0x16, 0xc5, 0xb1,
					0x6c, 0x5b, 0x16, 0xc5, 0xb1, 0x6c, 0x6d, 0xb6,
					0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x7f,
					0xff, 0x7f, 0xff, 0x7f, 0xff, 0x7f, 0xff, 0x7f,
					0xff, 0x7f, 0xff, 0x7c, 0xf9, 0xf3, 0xe7, 0xcf,
					0x9a, 0xd6, 0xb5, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd,
					0xdd, 0xdd, 0xdd, 0xdd, 0xef, 0x7b, 0xde, 0xf7,
					0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
					0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
					0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
					0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
					0xee, 0xee, 0xeb, 0x62, 0xd8, 0xb6, 0x2d, 0x8b,
					0x62, 0xd8, 0xb6, 0x2d, 0x8b, 0x62, 0xd8, 0xdb,
					0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6,
					0xff, 0xfe, 0xff, 0xfe, 0xff, 0xfe, 0xff, 0xfe,
					0xff, 0xfe, 0xff, 0xfe, 0xf9, 0xf3, 0xe7, 0xcf,
					0x9f, 0x35, 0xad, 0x6b, 0xbb, 0xbb, 0xbb, 0xbb,
					0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xde, 0xf7, 0xbd,
					0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
					0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
					0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
					0xbd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd,
					0xdd, 0xdd, 0xdd, 0xd6, 0xc5, 0xb1, 0x6c, 0x5b,
					0x16, 0xc5, 0xb1, 0x6c, 0x5b, 0x16, 0xc5, 0xb1,
					0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb,
					0x6d, 0xff, 0xfd, 0xff, 0xfd, 0xff, 0xfd, 0xff,
					0xfd, 0xff, 0xfd, 0xff, 0xfd, 0xf3, 0xe7, 0xcf,
				},
			},
			{
				Header: rtp.Header{
					Version:        2,
					Marker:         true,
					PayloadType:    14,
					SequenceNumber: 17647,
					Timestamp:      2289526357,
					SSRC:           0x9dbb7812,
				},
				Payload: []byte{
					0x00, 0x00, 0x03, 0x18, 0x9f, 0x3e, 0x6b, 0x5a,
					0xd7, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77,
					0x77, 0x77, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
					0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
					0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
					0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xbb, 0xbb,
					0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
					0xad, 0x8b, 0x62, 0xd8, 0xb6, 0x2d, 0x8b, 0x62,
					0xd8, 0xb6, 0x2d, 0x8b, 0x63, 0x6d, 0xb6, 0xdb,
					0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0xff, 0xfb,
					0xff, 0xfb, 0xff, 0xfb, 0xff, 0xfb, 0xff, 0xfb,
					0xff, 0xfb, 0xe7, 0xcf, 0x9f, 0x3e, 0x7c, 0xd6,
					0xb5, 0xae, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
					0xee, 0xee, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
					0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
					0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
					0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0x77,
					0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77, 0x77,
					0x77, 0x5b, 0x16, 0xc5, 0xb1, 0x6c, 0x5b, 0x16,
					0xc5, 0xb1, 0x6c, 0x5b, 0x16, 0xc6, 0xdb, 0x6d,
					0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb7, 0xff,
					0xf7, 0xff, 0xf7, 0xff, 0xf7, 0xff, 0xf7, 0xff,
					0xf7, 0xff, 0xf7, 0xcf, 0x9f, 0x3e, 0x7c, 0xf9,
					0xad, 0x6b, 0x5d, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd,
					0xdd, 0xdd, 0xdd, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
					0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd,
					0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde,
					0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xee,
					0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee, 0xee,
					0xee, 0xee, 0xb6, 0x2d, 0x8b, 0x62, 0xd8, 0xb6,
					0x2d, 0x8b, 0x62, 0xd8, 0xb6, 0x2d, 0x8d, 0xb6,
					0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6f,
					0xff, 0xef, 0xff, 0xef, 0xff, 0xef, 0xff, 0xef,
					0xff, 0xef, 0xff, 0xef, 0x9f, 0x3e, 0x7c, 0xf9,
					0xf3, 0x5a, 0xd6, 0xbb, 0xbb, 0xbb, 0xbb, 0xbb,
					0xbb, 0xbb, 0xbb, 0xbb, 0xbd, 0xef, 0x7b, 0xde,
					0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef,
					0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b, 0xde, 0xf7,
					0xbd, 0xef, 0x7b, 0xde, 0xf7, 0xbd, 0xef, 0x7b,
					0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd, 0xdd,
					0xdd, 0xdd, 0xdd, 0x6c, 0x5b, 0x16, 0xc5, 0xb1,
					0x6c, 0x5b, 0x16, 0xc5, 0xb1, 0x6c, 0x5b, 0x1b,
					0x6d, 0xb6, 0xdb, 0x6d, 0xb6, 0xdb, 0x6d, 0xb6,
					0xdf, 0xff, 0xdf, 0xff, 0xdf, 0xff, 0xdf, 0xff,
					0xdf, 0xff, 0xdf, 0xff, 0xdf, 0x3e, 0x7c, 0xf9,
					0xf3, 0xe6, 0xb5, 0xad,
				},
			},
		},
	},
}

func TestEncode(t *testing.T) {
	for _, ca := range cases {
		t.Run(ca.name, func(t *testing.T) {
			e := &Encoder{
				SSRC:                  uint32Ptr(0x9dbb7812),
				InitialSequenceNumber: uint16Ptr(0x44ed),
				InitialTimestamp:      uint32Ptr(0x88776655),
				PayloadMaxSize:        400,
			}
			err := e.Init()
			require.NoError(t, err)

			pkts, err := e.Encode(ca.frames, 0)
			require.NoError(t, err)
			require.Equal(t, ca.pkts, pkts)
		})
	}
}

func TestEncodeRandomInitialState(t *testing.T) {
	e := &Encoder{}
	err := e.Init()
	require.NoError(t, err)
	require.NotEqual(t, nil, e.SSRC)
	require.NotEqual(t, nil, e.InitialSequenceNumber)
	require.NotEqual(t, nil, e.InitialTimestamp)
}
