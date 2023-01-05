package h264

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDTSExtractor(t *testing.T) {
	type sequenceSample struct {
		nalus [][]byte
		pts   time.Duration
		dts   time.Duration
	}

	for _, ca := range []struct {
		name     string
		sequence []sequenceSample
	}{
		{
			"max_num_reorder_frames-based",
			[]sequenceSample{
				{
					[][]byte{
						{
							0x67, 0x64, 0x00, 0x28, 0xac, 0xd9, 0x40, 0x78,
							0x02, 0x27, 0xe5, 0xc0, 0x44, 0x00, 0x00, 0x03,
							0x00, 0x04, 0x00, 0x00, 0x03, 0x00, 0x28, 0x3c,
							0x60, 0xc6, 0x58,
						},
						{0x68, 0xeb, 0xe3, 0xcb, 0x22, 0xc0},
						{
							0x65, 0x88, 0x82, 0x00, 0x05, 0xbf, 0xfe, 0xf7,
							0xd3, 0x3f, 0xcc, 0xb2, 0xec, 0x9a, 0x24, 0xb5,
							0xe3, 0xa8, 0xf7, 0xa2, 0x9e, 0x26, 0x5f, 0x43,
							0x75, 0x25, 0x01, 0x9b, 0x96, 0xc4, 0xed, 0x3a,
							0x80, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x55,
							0xda, 0xf7, 0x10, 0xe5, 0xc4, 0x70, 0xe1, 0xfe,
							0x83, 0xc0, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x1f, 0xa0, 0x00, 0x00, 0x05, 0x68, 0x00,
							0x00, 0x03, 0x01, 0xc6, 0x00, 0x00, 0x03, 0x01,
							0x0c, 0x00, 0x00, 0x03, 0x00, 0xb1, 0x00, 0x00,
							0x03, 0x00, 0x8f, 0x80, 0x00, 0x00, 0x8a, 0x80,
							0x00, 0x00, 0x9d, 0x00, 0x00, 0x03, 0x00, 0xb2,
							0x00, 0x00, 0x03, 0x01, 0x1c, 0x00, 0x00, 0x03,
							0x01, 0x7c, 0x00, 0x00, 0x03, 0x02, 0xf0, 0x00,
							0x00, 0x04, 0x40, 0x00, 0x00, 0x08, 0x80, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x0b,
							0x78,
						},
					},
					0,
					-400 * time.Millisecond,
				},
				{
					[][]byte{
						{
							0x41, 0x9a, 0x24, 0x6c, 0x41, 0x4f, 0xfe, 0xd6,
							0x8c, 0xb0, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x6d, 0x40,
						},
					},
					800 * time.Millisecond,
					-200 * time.Millisecond,
				},
				{
					[][]byte{
						{
							0x41, 0x9e, 0x42, 0x78, 0x82, 0x1f, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x02,
							0x0f,
						},
					},
					400 * time.Millisecond,
					0,
				},
				{
					[][]byte{
						{
							0x01, 0x9e, 0x61, 0x74, 0x43, 0xff, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00,
							0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
							0x03, 0x00, 0x00, 0x03, 0x00, 0x00, 0x04, 0x9c,
						},
					},
					200 * time.Millisecond,
					200 * time.Millisecond,
				},
			},
		},
		{
			"sei-based",
			[]sequenceSample{
				{
					[][]byte{
						{6, 0, 7, 128, 117, 48, 0, 0, 3, 0, 64, 128}, // SEI (buffering period)
						{6, 1, 4, 0, 0, 8, 16, 128},                  // SEI (pic timing)
						{ // SPS
							103, 100, 0, 42, 172, 44, 172, 7,
							128, 34, 126, 92, 5, 168, 8, 8,
							10, 0, 0, 7, 208, 0, 3, 169,
							129, 192, 0, 0, 76, 75, 0, 0,
							38, 37, 173, 222, 92, 20,
						},
					},
					0,
					-16666666 * time.Nanosecond,
				},
				{
					[][]byte{
						{6, 1, 4, 0, 2, 32, 16, 128}, // SEI
					},
					66666666 * time.Nanosecond,
					0,
				},
				{
					[][]byte{
						{6, 1, 4, 0, 4, 0, 16, 128}, // SEI
					},
					16666666 * time.Nanosecond,
					16666666 * time.Nanosecond,
				},
				{
					[][]byte{
						{6, 1, 4, 0, 6, 0, 16, 128}, // SEI
					},
					33333333 * time.Nanosecond,
					33333333 * time.Nanosecond,
				},
			},
		},
		{
			"sei-based nvenc",
			[]sequenceSample{
				{
					[][]byte{
						{ // SPS
							103, 100, 0, 42, 172, 44, 172, 7,
							128, 34, 126, 92, 5, 168, 8, 8,
							10, 0, 0, 7, 208, 0, 3, 169,
							129, 192, 0, 0, 76, 75, 0, 0,
							38, 37, 173, 222, 92, 20,
						},
						{6, 0, 7, 128, 175, 199, 128, 0, 0, 192, 128}, // SEI
						{6, 1, 4, 0, 120, 40, 16, 128},                // SEI
						{5},                                           // IDR
					},
					999 * time.Millisecond,
					982333334 * time.Nanosecond,
				},
				{[][]byte{{6, 1, 4, 0, 2, 40, 16, 128}}, 1083 * time.Millisecond, 999666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 4, 0, 16, 128}}, 1016 * time.Millisecond, 1016 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 6, 0, 16, 128}}, 1033 * time.Millisecond, 1033 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 8, 0, 16, 128}}, 1050 * time.Millisecond, 1050 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 10, 0, 16, 128}}, 1066 * time.Millisecond, 1066 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 12, 40, 16, 128}}, 1166 * time.Millisecond, 1082666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 14, 0, 16, 128}}, 1100 * time.Millisecond, 1100 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 16, 0, 16, 128}}, 1116 * time.Millisecond, 1116 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 18, 0, 16, 128}}, 1133 * time.Millisecond, 1133 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 20, 0, 16, 128}}, 1150 * time.Millisecond, 1150 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 22, 40, 16, 128}}, 1249 * time.Millisecond, 1165666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 24, 0, 16, 128}}, 1183 * time.Millisecond, 1183 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 26, 0, 16, 128}}, 1200 * time.Millisecond, 1200 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 28, 0, 16, 128}}, 1216 * time.Millisecond, 1216 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 30, 0, 16, 128}}, 1233 * time.Millisecond, 1233 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 32, 40, 16, 128}}, 1333 * time.Millisecond, 1249666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 34, 0, 16, 128}}, 1266 * time.Millisecond, 1266 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 36, 0, 16, 128}}, 1283 * time.Millisecond, 1283 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 38, 0, 16, 128}}, 1300 * time.Millisecond, 1300 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 40, 0, 16, 128}}, 1316 * time.Millisecond, 1316 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 42, 40, 16, 128}}, 1416 * time.Millisecond, 1332666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 44, 0, 16, 128}}, 1350 * time.Millisecond, 1350 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 46, 0, 16, 128}}, 1366 * time.Millisecond, 1366 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 48, 0, 16, 128}}, 1383 * time.Millisecond, 1383 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 50, 0, 16, 128}}, 1400 * time.Millisecond, 1400 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 52, 40, 16, 128}}, 1499 * time.Millisecond, 1415666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 54, 0, 16, 128}}, 1433 * time.Millisecond, 1433 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 56, 0, 16, 128}}, 1450 * time.Millisecond, 1450 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 58, 0, 16, 128}}, 1466 * time.Millisecond, 1466 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 60, 0, 16, 128}}, 1483 * time.Millisecond, 1483 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 62, 40, 16, 128}}, 1583 * time.Millisecond, 1499666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 64, 0, 16, 128}}, 1516 * time.Millisecond, 1516 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 66, 0, 16, 128}}, 1533 * time.Millisecond, 1533 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 68, 0, 16, 128}}, 1550 * time.Millisecond, 1550 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 70, 0, 16, 128}}, 1566 * time.Millisecond, 1566 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 72, 40, 16, 128}}, 1666 * time.Millisecond, 1582666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 74, 0, 16, 128}}, 1600 * time.Millisecond, 1600 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 76, 0, 16, 128}}, 1616 * time.Millisecond, 1616 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 78, 0, 16, 128}}, 1633 * time.Millisecond, 1633 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 80, 0, 16, 128}}, 1650 * time.Millisecond, 1650 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 82, 40, 16, 128}}, 1749 * time.Millisecond, 1665666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 84, 0, 16, 128}}, 1683 * time.Millisecond, 1683 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 86, 0, 16, 128}}, 1700 * time.Millisecond, 1700 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 88, 0, 16, 128}}, 1716 * time.Millisecond, 1716 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 90, 0, 16, 128}}, 1733 * time.Millisecond, 1733 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 92, 40, 16, 128}}, 1833 * time.Millisecond, 1749666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 94, 0, 16, 128}}, 1766 * time.Millisecond, 1766 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 96, 0, 16, 128}}, 1783 * time.Millisecond, 1783 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 98, 0, 16, 128}}, 1800 * time.Millisecond, 1800 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 100, 0, 16, 128}}, 1816 * time.Millisecond, 1816 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 102, 40, 16, 128}}, 1916 * time.Millisecond, 1832666667 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 104, 0, 16, 128}}, 1850 * time.Millisecond, 1850 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 106, 0, 16, 128}}, 1866 * time.Millisecond, 1866 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 108, 0, 16, 128}}, 1883 * time.Millisecond, 1883 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 110, 0, 16, 128}}, 1900 * time.Millisecond, 1900 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 112, 32, 16, 128}}, 1982 * time.Millisecond, 1915333334 * time.Nanosecond},
				{[][]byte{{6, 1, 4, 0, 114, 0, 16, 128}}, 1933 * time.Millisecond, 1933 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 116, 0, 16, 128}}, 1950 * time.Millisecond, 1950 * time.Millisecond},
				{[][]byte{{6, 1, 4, 0, 118, 0, 16, 128}}, 1966 * time.Millisecond, 1966 * time.Millisecond},
				{
					[][]byte{
						{ // SPS
							103, 100, 0, 42, 172, 44, 172, 7,
							128, 34, 126, 92, 5, 168, 8, 8,
							10, 0, 0, 7, 208, 0, 3, 169,
							129, 192, 0, 0, 76, 75, 0, 0,
							38, 37, 173, 222, 92, 20,
						},
						{6, 0, 7, 128, 175, 199, 128, 0, 0, 192, 128}, // SEI
						{6, 1, 4, 0, 120, 40, 16, 128},                // SEI
						{5},                                           // IDR
					},
					1999 * time.Millisecond,
					1982333334 * time.Nanosecond,
				},
			},
		},
		{
			"no timing info",
			[]sequenceSample{
				{
					[][]byte{
						{ // SPS
							0x27, 0x64, 0x00, 0x2a, 0xac, 0x52, 0x14, 0x07,
							0x80, 0x22, 0x7e, 0x5f, 0xfc, 0x00, 0x04, 0x00,
							0x05, 0xa8, 0x08, 0x08, 0x0d, 0xb6, 0x15, 0xaf,
							0x7b, 0xe0, 0x20,
						},
						{
							// IDR
							byte(NALUTypeIDR),
						},
					},
					1 * time.Second,
					1 * time.Second,
				},
			},
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			ex := NewDTSExtractor()
			for _, sample := range ca.sequence {
				dts, err := ex.Extract(sample.nalus, sample.pts)
				require.NoError(t, err)
				require.Equal(t, sample.dts, dts)
			}
		})
	}
}
