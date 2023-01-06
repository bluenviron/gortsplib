package h264

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDTSExtractor(t *testing.T) {
	type sequenceSample struct {
		nalus [][]byte
		dts   time.Duration
		pts   time.Duration
	}

	for _, ca := range []struct {
		name     string
		sequence []sequenceSample
	}{
		{
			"with timing info",
			[]sequenceSample{
				{
					[][]byte{
						{ // SPS
							0x67, 0x64, 0x00, 0x28, 0xac, 0xd9, 0x40, 0x78,
							0x02, 0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00,
							0x04, 0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60,
							0xc6, 0x58,
						},
						{ // IDR
							0x65, 0x88, 0x84, 0x00, 0x33, 0xff,
						},
					},
					33333333333 * time.Nanosecond,
					33333333333 * time.Nanosecond,
				},
				{
					[][]byte{{0x41, 0x9a, 0x21, 0x6c, 0x45, 0xff}},
					33366666666 * time.Nanosecond,
					33366666666 * time.Nanosecond,
				},
				{
					[][]byte{{0x41, 0x9a, 0x42, 0x3c, 0x21, 0x93}},
					33400000000 * time.Nanosecond,
					33400000000 * time.Nanosecond,
				},
				{
					[][]byte{{0x41, 0x9a, 0x63, 0x49, 0xe1, 0x0f}},
					33433333333 * time.Nanosecond,
					33433333333 * time.Nanosecond,
				},
				{
					[][]byte{{0x41, 0x9a, 0x86, 0x49, 0xe1, 0x0f}},
					33434333333 * time.Nanosecond,
					33533333333 * time.Nanosecond,
				},
				{
					[][]byte{{0x41, 0x9e, 0xa5, 0x42, 0x7f, 0xf9}},
					33435333333 * time.Nanosecond,
					33500000000 * time.Nanosecond,
				},
				{
					[][]byte{{0x01, 0x9e, 0xc4, 0x69, 0x13, 0xff}},
					33466666666 * time.Nanosecond,
					33466666666 * time.Nanosecond,
				},
				{
					[][]byte{{0x41, 0x9a, 0xc8, 0x4b, 0xa8, 0x42}},
					33499999999 * time.Nanosecond,
					33600000000 * time.Nanosecond,
				},
			},
		},
		{
			"no timing info",
			[]sequenceSample{
				{
					[][]byte{
						{ // SPS
							0x27, 0x64, 0x00, 0x20, 0xac, 0x52, 0x18, 0x0f,
							0x01, 0x17, 0xef, 0xff, 0x00, 0x01, 0x00, 0x01,
							0x6a, 0x02, 0x02, 0x03, 0x6d, 0x85, 0x6b, 0xde,
							0xf8, 0x08,
						},
						{
							// IDR
							0x25, 0xb8, 0x08, 0x02, 0x1f, 0xff,
						},
					},
					850000000 * time.Nanosecond,
					850000000 * time.Nanosecond,
				},
				{
					[][]byte{{0x21, 0xe1, 0x05, 0xc7, 0x38, 0xbf}},
					866666667 * time.Nanosecond,
					866666667 * time.Nanosecond,
				},
				{
					[][]byte{{0x21, 0xe2, 0x09, 0xa1, 0xce, 0x0b}},
					883333334 * time.Nanosecond,
					883333334 * time.Nanosecond,
				},
				{
					[][]byte{{0x21, 0xe3, 0x0d, 0xb1, 0xce, 0x02}},
					900000000 * time.Nanosecond,
					900000000 * time.Nanosecond,
				},
				{
					[][]byte{{0x21, 0xe4, 0x11, 0x90, 0x73, 0x80}},
					916666667 * time.Nanosecond,
					916666667 * time.Nanosecond,
				},
				{
					[][]byte{{0x21, 0xe5, 0x19, 0x0e, 0x70, 0x01}},
					917666667 * time.Nanosecond,
					950000000 * time.Nanosecond,
				},
				{
					[][]byte{{0x01, 0xa9, 0x85, 0x7c, 0x93, 0xff}},
					933333334 * time.Nanosecond,
					933333334 * time.Nanosecond,
				},
				{
					[][]byte{{0x21, 0xe6, 0x1d, 0x0e, 0x70, 0x01}},
					950000000 * time.Nanosecond,
					966666667 * time.Nanosecond,
				},
				{
					[][]byte{{0x21, 0xe7, 0x21, 0x0e, 0x70, 0x01}},
					966666667 * time.Nanosecond,
					983333334 * time.Nanosecond,
				},
				{
					[][]byte{{0x21, 0xe8, 0x25, 0x0e, 0x70, 0x01}},
					983333333 * time.Nanosecond,
					1000000000 * time.Nanosecond,
				},
				{
					[][]byte{{0x21, 0xe9, 0x29, 0x0e, 0x70, 0x01}},
					1000000000 * time.Nanosecond,
					1016666667 * time.Nanosecond,
				},
				{
					[][]byte{{0x21, 0xea, 0x31, 0x0e, 0x70, 0x01}},
					1016666666 * time.Nanosecond,
					1050000000 * time.Nanosecond,
				},
				{
					[][]byte{{0x01, 0xaa, 0xcb, 0x7c, 0x93, 0xff}},
					1033333334 * time.Nanosecond,
					1033333334 * time.Nanosecond,
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
