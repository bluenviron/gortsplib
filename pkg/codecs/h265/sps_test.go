//go:build go1.18
// +build go1.18

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
		fps    float64
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
					GeneralProgressiveSourceFlag:   true,
					GeneralFrameOnlyConstraintFlag: true,
					GeneralLevelIdc:                120,
				},
				ChromaFormatIdc:                      1,
				PicWidthInLumaSamples:                1920,
				PicHeightInLumaSamples:               1080,
				Log2MaxPicOrderCntLsbMinus4:          4,
				SubLayerOrderingInfoPresentFlag:      true,
				MaxDecPicBufferingMinus1:             []uint32{5},
				MaxNumReorderPics:                    []uint32{2},
				MaxLatencyIncreasePlus1:              []uint32{5},
				Log2DiffMaxMinLumaCodingBlockSize:    3,
				Log2DiffMaxMinLumaTransformBlockSize: 3,
				SampleAdaptiveOffsetEnabledFlag:      true,
				TemporalMvpEnabledFlag:               true,
				StrongIntraSmoothingEnabledFlag:      true,
				VUI: &SPS_VUI{
					TimingInfo: &SPS_TimingInfo{
						NumUnitsInTick: 1,
						TimeScale:      30,
					},
				},
			},
			1920,
			1080,
			30,
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
					GeneralProgressiveSourceFlag:   true,
					GeneralFrameOnlyConstraintFlag: true,
					GeneralLevelIdc:                120,
				},
				ChromaFormatIdc:                      1,
				PicWidthInLumaSamples:                1920,
				PicHeightInLumaSamples:               800,
				Log2MaxPicOrderCntLsbMinus4:          4,
				SubLayerOrderingInfoPresentFlag:      true,
				MaxDecPicBufferingMinus1:             []uint32{4},
				MaxNumReorderPics:                    []uint32{2},
				MaxLatencyIncreasePlus1:              []uint32{5},
				Log2DiffMaxMinLumaCodingBlockSize:    3,
				Log2DiffMaxMinLumaTransformBlockSize: 3,
				SampleAdaptiveOffsetEnabledFlag:      true,
				TemporalMvpEnabledFlag:               true,
				StrongIntraSmoothingEnabledFlag:      true,
				VUI: &SPS_VUI{
					AspectRatioInfoPresentFlag:   true,
					AspectRatioIdc:               1,
					VideoSignalTypePresentFlag:   true,
					VideoFormat:                  5,
					ColourDescriptionPresentFlag: true,
					ColourPrimaries:              1,
					TransferCharacteristics:      1,
					MatrixCoefficients:           1,
					TimingInfo: &SPS_TimingInfo{
						NumUnitsInTick: 1001,
						TimeScale:      24000,
					},
				},
			},
			1920,
			800,
			23.976023976023978,
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
					GeneralProgressiveSourceFlag:      true,
					GeneralFrameOnlyConstraintFlag:    true,
					GeneralMax12bitConstraintFlag:     true,
					GeneralLowerBitRateConstraintFlag: true,
					GeneralLevelIdc:                   93,
				},
				ChromaFormatIdc:                      3,
				PicWidthInLumaSamples:                1280,
				PicHeightInLumaSamples:               720,
				BitDepthLumaMinus8:                   4,
				BitDepthChromaMinus8:                 4,
				Log2MaxPicOrderCntLsbMinus4:          4,
				SubLayerOrderingInfoPresentFlag:      true,
				MaxDecPicBufferingMinus1:             []uint32{2},
				MaxNumReorderPics:                    []uint32{0},
				MaxLatencyIncreasePlus1:              []uint32{1},
				Log2MinLumaCodingBlockSizeMinus3:     1,
				Log2DiffMaxMinLumaCodingBlockSize:    1,
				Log2DiffMaxMinLumaTransformBlockSize: 3,
				TemporalMvpEnabledFlag:               true,
				StrongIntraSmoothingEnabledFlag:      true,
				VUI: &SPS_VUI{
					AspectRatioInfoPresentFlag:   true,
					AspectRatioIdc:               255,
					SarWidth:                     1,
					SarHeight:                    1,
					VideoSignalTypePresentFlag:   true,
					VideoFormat:                  5,
					ColourDescriptionPresentFlag: true,
					ColourPrimaries:              1,
					TransferCharacteristics:      1,
					MatrixCoefficients:           1,
					TimingInfo: &SPS_TimingInfo{
						NumUnitsInTick: 1,
						TimeScale:      30,
					},
				},
			},
			1280,
			720,
			30,
		},
		{
			"10 bit",
			[]byte{
				0x42, 0x01, 0x01, 0x22, 0x20, 0x00, 0x00, 0x03,
				0x00, 0x90, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
				0x00, 0x78, 0xa0, 0x03, 0xc0, 0x80, 0x10, 0xe4,
				0xd9, 0x66, 0x66, 0x92, 0x4c, 0xaf, 0x01, 0x01,
				0x00, 0x00, 0x03, 0x00, 0x64, 0x00, 0x00, 0x0b,
				0xb5, 0x08,
			},
			SPS{
				TemporalIDNestingFlag: true,
				ProfileTierLevel: SPS_ProfileTierLevel{
					GeneralTierFlag:   1,
					GeneralProfileIdc: 2,
					GeneralProfileCompatibilityFlag: [32]bool{
						false, false, true, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
					},
					GeneralProgressiveSourceFlag:   true,
					GeneralFrameOnlyConstraintFlag: true,
					GeneralLevelIdc:                120,
				},
				ChromaFormatIdc:                      1,
				PicWidthInLumaSamples:                1920,
				PicHeightInLumaSamples:               1080,
				BitDepthLumaMinus8:                   2,
				BitDepthChromaMinus8:                 2,
				Log2MaxPicOrderCntLsbMinus4:          4,
				SubLayerOrderingInfoPresentFlag:      true,
				MaxDecPicBufferingMinus1:             []uint32{5},
				MaxNumReorderPics:                    []uint32{2},
				MaxLatencyIncreasePlus1:              []uint32{5},
				Log2DiffMaxMinLumaCodingBlockSize:    3,
				Log2DiffMaxMinLumaTransformBlockSize: 3,
				SampleAdaptiveOffsetEnabledFlag:      true,
				TemporalMvpEnabledFlag:               true,
				StrongIntraSmoothingEnabledFlag:      true,
				VUI: &SPS_VUI{
					AspectRatioInfoPresentFlag: true,
					AspectRatioIdc:             1,
					TimingInfo: &SPS_TimingInfo{
						NumUnitsInTick: 100,
						TimeScale:      2997,
					},
				},
			},
			1920,
			1080,
			29.97,
		},
		{
			"nvenc",
			[]byte{
				0x42, 0x01, 0x01, 0x01, 0x40, 0x00, 0x00, 0x03,
				0x00, 0x00, 0x03, 0x00, 0x00, 0x03, 0x00, 0x00,
				0x03, 0x00, 0x7b, 0xa0, 0x03, 0xc0, 0x80, 0x11,
				0x07, 0xcb, 0x96, 0xb4, 0xa4, 0x25, 0x92, 0xe3,
				0x01, 0x6a, 0x02, 0x02, 0x02, 0x08, 0x00, 0x00,
				0x03, 0x00, 0x08, 0x00, 0x00, 0x03, 0x01, 0xe3,
				0x00, 0x2e, 0xf2, 0x88, 0x00, 0x07, 0x27, 0x0c,
				0x00, 0x00, 0x98, 0x96, 0x82,
			},
			SPS{
				TemporalIDNestingFlag: true,
				ProfileTierLevel: SPS_ProfileTierLevel{
					GeneralProfileIdc: 1,
					GeneralProfileCompatibilityFlag: [32]bool{
						false, true, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
						false, false, false, false, false, false, false, false,
					},
					GeneralLevelIdc: 123,
				},
				ChromaFormatIdc:        1,
				PicWidthInLumaSamples:  1920,
				PicHeightInLumaSamples: 1088,
				ConformanceWindow: &SPS_ConformanceWindow{
					BottomOffset: 4,
				},
				Log2MaxPicOrderCntLsbMinus4:          4,
				SubLayerOrderingInfoPresentFlag:      true,
				MaxDecPicBufferingMinus1:             []uint32{1},
				MaxNumReorderPics:                    []uint32{0},
				MaxLatencyIncreasePlus1:              []uint32{0},
				Log2MinLumaCodingBlockSizeMinus3:     1,
				Log2DiffMaxMinLumaCodingBlockSize:    1,
				Log2DiffMaxMinLumaTransformBlockSize: 3,
				MaxTransformHierarchyDepthInter:      3,
				AmpEnabledFlag:                       true,
				SampleAdaptiveOffsetEnabledFlag:      true,
				ShortTermRefPicSets: []*SPS_ShortTermRefPicSet{{
					NumNegativePics:     1,
					DeltaPocS0Minus1:    []uint32{0},
					UsedByCurrPicS0Flag: []bool{true},
				}},
				VUI: &SPS_VUI{
					AspectRatioInfoPresentFlag:   true,
					AspectRatioIdc:               1,
					VideoSignalTypePresentFlag:   true,
					VideoFormat:                  5,
					ColourDescriptionPresentFlag: true,
					ColourPrimaries:              1,
					TransferCharacteristics:      1,
					MatrixCoefficients:           1,
					TimingInfo: &SPS_TimingInfo{
						NumUnitsInTick: 1,
						TimeScale:      60,
					},
				},
			},
			1920,
			1080,
			60,
		},
		{
			"avigilon",
			[]byte{
				0x42, 0x01, 0x01, 0x01, 0x60, 0x00, 0x00, 0x03,
				0x00, 0x80, 0x00, 0x00, 0x03, 0x00, 0x00, 0x03,
				0x00, 0x96, 0xa0, 0x01, 0x80, 0x20, 0x06, 0xc1,
				0xfe, 0x36, 0xbb, 0xb5, 0x37, 0x77, 0x25, 0xd6,
				0x02, 0xdc, 0x04, 0x04, 0x04, 0x10, 0x00, 0x00,
				0x3e, 0x80, 0x00, 0x04, 0x26, 0x87, 0x21, 0xde,
				0xe5, 0x10, 0x01, 0x6e, 0x20, 0x00, 0x66, 0xff,
				0x00, 0x0b, 0x71, 0x00, 0x03, 0x37, 0xf8, 0x80,
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
					GeneralProgressiveSourceFlag: true,
					GeneralLevelIdc:              150,
				},
				ChromaFormatIdc:                      1,
				PicWidthInLumaSamples:                3072,
				PicHeightInLumaSamples:               1728,
				ConformanceWindow:                    &SPS_ConformanceWindow{},
				Log2MaxPicOrderCntLsbMinus4:          12,
				SubLayerOrderingInfoPresentFlag:      true,
				MaxDecPicBufferingMinus1:             []uint32{1},
				MaxNumReorderPics:                    []uint32{0},
				MaxLatencyIncreasePlus1:              []uint32{0},
				Log2DiffMaxMinLumaCodingBlockSize:    2,
				SampleAdaptiveOffsetEnabledFlag:      true,
				PcmEnabledFlag:                       true,
				PcmSampleBitDepthLumaMinus1:          7,
				PcmSampleBitDepthChromaMinus1:        7,
				Log2DiffMaxMinLumaTransformBlockSize: 2,
				MaxTransformHierarchyDepthInter:      1,
				Log2MinPcmLumaCodingBlockSizeMinus3:  2,
				ShortTermRefPicSets: []*SPS_ShortTermRefPicSet{
					{
						NumNegativePics:     1,
						DeltaPocS0Minus1:    []uint32{0},
						UsedByCurrPicS0Flag: []bool{true},
					},
				},
				TemporalMvpEnabledFlag: true,
				VUI: &SPS_VUI{
					AspectRatioInfoPresentFlag:   true,
					AspectRatioIdc:               1,
					VideoSignalTypePresentFlag:   true,
					VideoFormat:                  5,
					VideoFullRangeFlag:           true,
					ColourDescriptionPresentFlag: true,
					ColourPrimaries:              1,
					TransferCharacteristics:      1,
					MatrixCoefficients:           1,
					TimingInfo: &SPS_TimingInfo{
						NumUnitsInTick: 1000,
						TimeScale:      17000,
					},
				},
			},
			3072,
			1728,
			17,
		},
	} {
		t.Run(ca.name, func(t *testing.T) {
			var sps SPS
			err := sps.Unmarshal(ca.byts)
			require.NoError(t, err)
			require.Equal(t, ca.sps, sps)
			require.Equal(t, ca.width, sps.Width())
			require.Equal(t, ca.height, sps.Height())
			require.Equal(t, ca.fps, sps.FPS())
		})
	}
}

func FuzzSPSUnmarshal(f *testing.F) {
	f.Fuzz(func(t *testing.T, b []byte) {
		var sps SPS
		sps.Unmarshal(b)
	})
}
