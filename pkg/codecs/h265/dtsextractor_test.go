package h265

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
			"no timing info",
			[]sequenceSample{
				{
					[][]byte{
						{ // SPS
							0x42, 0x01, 0x01, 0x02, 0x20, 0x00, 0x00, 0x03,
							0x00, 0xb0, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
							0x00, 0x7b, 0xa0, 0x07, 0x82, 0x00, 0x88, 0x7d,
							0xb6, 0x71, 0x8b, 0x92, 0x44, 0x80, 0x53, 0x88,
							0x88, 0x92, 0xcf, 0x24, 0xa6, 0x92, 0x72, 0xc9,
							0x12, 0x49, 0x22, 0xdc, 0x91, 0xaa, 0x48, 0xfc,
							0xa2, 0x23, 0xff, 0x00, 0x01, 0x00, 0x01, 0x6a,
							0x02, 0x02, 0x02, 0x01,
						},
						{ // PPS
							0x44, 0x01, 0xc0, 0x25, 0x2f, 0x05, 0x32, 0x40,
						},
						{
							byte(NALUType_CRA_NUT) << 1,
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

func FuzzDTSExtractor(f *testing.F) {
	sps := []byte{
		0x42, 0x01, 0x01, 0x01, 0x60, 0x00, 0x00, 0x03,
		0x00, 0x90, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
		0x00, 0x78, 0xa0, 0x03, 0xc0, 0x80, 0x10, 0xe5,
		0x96, 0x66, 0x69, 0x24, 0xca, 0xe0, 0x10, 0x00,
		0x00, 0x03, 0x00, 0x10, 0x00, 0x00, 0x03, 0x01,
		0xe0, 0x80,
	}

	pps := []byte{
		0x44, 0x01, 0xc1, 0x72, 0xb4, 0x62, 0x40,
	}

	ex := NewDTSExtractor()
	f.Fuzz(func(t *testing.T, b []byte, p uint64) {
		if len(b) < 1 {
			return
		}
		ex.Extract([][]byte{sps, pps, b}, time.Duration(p))
	})
}
