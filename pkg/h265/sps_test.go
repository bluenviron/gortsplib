package h265

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSPSUnmarshal(t *testing.T) {
	for _, ca := range []struct {
		name   string
		byts   []byte
		sps    SPS
		width  int
		height int
	}{
		{
			"1920x1080",
			[]byte{
				0x42, 0x01, 0x01, 0x01, 0x60, 0x00, 0x00, 0x03,
				0x00, 0x90, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
				0x00, 0x78, 0xa0, 0x03, 0xc0, 0x80, 0x10, 0xe5,
				0x96, 0x66, 0x69, 0x24, 0xca, 0xe0, 0x10, 0x00,
				0x00, 0x03, 0x00, 0x10, 0x00, 0x00, 0x03, 0x01,
				0xe0, 0x80,
			},
			SPS{
				TemporalIDNestingFlag: true,
				ProfileTierLevel: SPS_ProfileTierLevel{
					GeneralProfileIdc: 1,
					GeneralProfileCompatibilityFlag: [32]bool{
						false, true, true, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
					},
					ProgressiveSourceFlag:   true,
					FrameOnlyConstraintFlag: true,
					LevelIdc:                120,
				},
				ChromaFormatIdc:             1,
				PicWidthInLumaSamples:       1920,
				PicHeightInLumaSamples:      1080,
				Log2MaxPicOrderCntLsbMinus4: 4,
			},
			1920,
			1080,
		},
		{
			"1920x800",
			[]byte{
				0x42, 0x01, 0x01, 0x01, 0x60, 0x00, 0x00, 0x03,
				0x00, 0x90, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
				0x00, 0x78, 0xa0, 0x03, 0xc0, 0x80, 0x32, 0x16,
				0x59, 0x59, 0xa4, 0x93, 0x2b, 0xc0, 0x5a, 0x80,
				0x80, 0x80, 0x82, 0x00, 0x00, 0x07, 0xd2, 0x00,
				0x00, 0xbb, 0x80, 0x10,
			},
			SPS{
				TemporalIDNestingFlag: true,
				ProfileTierLevel: SPS_ProfileTierLevel{
					GeneralProfileIdc: 1,
					GeneralProfileCompatibilityFlag: [32]bool{
						false, true, true, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
					},
					ProgressiveSourceFlag:   true,
					FrameOnlyConstraintFlag: true,
					LevelIdc:                120,
				},
				ChromaFormatIdc:             1,
				PicWidthInLumaSamples:       1920,
				PicHeightInLumaSamples:      800,
				Log2MaxPicOrderCntLsbMinus4: 4,
			},
			1920,
			800,
		},
		{
			"1280x720",
			[]byte{
				0x42, 0x01, 0x01, 0x04, 0x08, 0x00, 0x00, 0x03,
				0x00, 0x98, 0x08, 0x00, 0x00, 0x03, 0x00, 0x00,
				0x5d, 0x90, 0x00, 0x50, 0x10, 0x05, 0xa2, 0x29,
				0x4b, 0x74, 0x94, 0x98, 0x5f, 0xfe, 0x00, 0x02,
				0x00, 0x02, 0xd4, 0x04, 0x04, 0x04, 0x10, 0x00,
				0x00, 0x03, 0x00, 0x10, 0x00, 0x00, 0x03, 0x01,
				0xe0, 0x80,
			},
			SPS{
				TemporalIDNestingFlag: true,
				ProfileTierLevel: SPS_ProfileTierLevel{
					GeneralProfileIdc: 4,
					GeneralProfileCompatibilityFlag: [32]bool{
						false, false, false, false, true, false, false, false,
						false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
					},
					ProgressiveSourceFlag:      true,
					FrameOnlyConstraintFlag:    true,
					Max12bitConstraintFlag:     true,
					LowerBitRateConstraintFlag: true,
					LevelIdc:                   93,
				},
				ChromaFormatIdc:             3,
				PicWidthInLumaSamples:       1280,
				PicHeightInLumaSamples:      720,
				BitDepthLumaMinus8:          4,
				BitDepthChromaMinus8:        4,
				Log2MaxPicOrderCntLsbMinus4: 4,
			},
			1280,
			720,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var sps SPS
			err := sps.Unmarshal(ca.byts)
			require.NoError(t, err)
			require.Equal(t, ca.sps, sps)
			require.Equal(t, ca.width, sps.Width())
			require.Equal(t, ca.height, sps.Height())
		})
	}
}
