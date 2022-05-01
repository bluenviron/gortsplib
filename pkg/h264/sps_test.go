package h264

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
			"352x288",
			[]byte{
				0x67, 0x64, 0x00, 0x0c, 0xac, 0x3b, 0x50, 0xb0,
				0x4b, 0x42, 0x00, 0x00, 0x03, 0x00, 0x02, 0x00,
				0x00, 0x03, 0x00, 0x3d, 0x08,
			},
			SPS{
				ProfileIdc:                     100,
				LevelIdc:                       12,
				ChromeFormatIdc:                1,
				Log2MaxFrameNumMinus4:          6,
				PicOrderCntType:                2,
				MaxNumRefFrames:                1,
				GapsInFrameNumValueAllowedFlag: true,
				PicWidthInMbsMinus1:            21,
				PicHeightInMbsMinus1:           17,
				FrameMbsOnlyFlag:               true,
				Direct8x8InferenceFlag:         true,
				VUI: &VUI{
					TimingInfoPresentFlag: true,
					NumUnitsInTick:        1,
					TimeScale:             30,
					FixedFrameRateFlag:    true,
				},
			},
			352,
			288,
			15,
		},
		{
			"1280x720",
			[]byte{
				0x67, 0x64, 0x00, 0x1f, 0xac, 0xd9, 0x40, 0x50,
				0x05, 0xbb, 0x01, 0x6c, 0x80, 0x00, 0x00, 0x03,
				0x00, 0x80, 0x00, 0x00, 0x1e, 0x07, 0x8c, 0x18,
				0xcb,
			},
			SPS{
				ProfileIdc:                  100,
				LevelIdc:                    31,
				ChromeFormatIdc:             1,
				Log2MaxPicOrderCntLsbMinus4: 2,
				MaxNumRefFrames:             4,
				PicWidthInMbsMinus1:         79,
				PicHeightInMbsMinus1:        44,
				FrameMbsOnlyFlag:            true,
				Direct8x8InferenceFlag:      true,
				VUI: &VUI{
					AspectRatioInfoPresentFlag:         true,
					AspectRatioIdc:                     1,
					VideoSignalTypePresentFlag:         true,
					VideoFormat:                        5,
					VideoFullRangeFlag:                 true,
					TimingInfoPresentFlag:              true,
					NumUnitsInTick:                     1,
					TimeScale:                          60,
					BitstreamRestrictionFlag:           true,
					MotionVectorsOverPicBoundariesFlag: true,
					Log2MaxMvLengthHorizontal:          11,
					Log2MaxMvLengthVertical:            11,
					MaxNumReorderFrames:                2,
					MaxDecFrameBuffering:               4,
				},
			},
			1280,
			720,
			30,
		},
		{
			"1920x1080 baseline",
			[]byte{
				0x67, 0x42, 0xc0, 0x28, 0xd9, 0x00, 0x78, 0x02,
				0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00, 0x04,
				0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60, 0xc9, 0x20,
			},
			SPS{
				ProfileIdc:             66,
				ConstraintSet0Flag:     true,
				ConstraintSet1Flag:     true,
				LevelIdc:               40,
				PicOrderCntType:        2,
				MaxNumRefFrames:        3,
				PicWidthInMbsMinus1:    119,
				PicHeightInMbsMinus1:   67,
				FrameMbsOnlyFlag:       true,
				Direct8x8InferenceFlag: true,
				FrameCroppingFlag:      true,
				FrameCropBottomOffset:  4,
				VUI: &VUI{
					TimingInfoPresentFlag:              true,
					NumUnitsInTick:                     1,
					TimeScale:                          60,
					BitstreamRestrictionFlag:           true,
					MotionVectorsOverPicBoundariesFlag: true,
					Log2MaxMvLengthHorizontal:          11,
					Log2MaxMvLengthVertical:            11,
					MaxDecFrameBuffering:               3,
				},
			},
			1920,
			1080,
			30,
		},
		{
			"1920x1080 nvidia",
			[]byte{
				0x67, 0x64, 0x00, 0x28, 0xac, 0xd9, 0x40, 0x78,
				0x02, 0x27, 0xe5, 0x84, 0x00, 0x00, 0x03, 0x00,
				0x04, 0x00, 0x00, 0x03, 0x00, 0xf0, 0x3c, 0x60,
				0xc6, 0x58,
			},
			SPS{
				ProfileIdc:                  100,
				LevelIdc:                    40,
				ChromeFormatIdc:             1,
				Log2MaxPicOrderCntLsbMinus4: 2,
				MaxNumRefFrames:             4,
				PicWidthInMbsMinus1:         119,
				PicHeightInMbsMinus1:        67,
				FrameMbsOnlyFlag:            true,
				Direct8x8InferenceFlag:      true,
				FrameCroppingFlag:           true,
				FrameCropBottomOffset:       4,
				VUI: &VUI{
					TimingInfoPresentFlag:              true,
					NumUnitsInTick:                     1,
					TimeScale:                          60,
					BitstreamRestrictionFlag:           true,
					MotionVectorsOverPicBoundariesFlag: true,
					Log2MaxMvLengthHorizontal:          11,
					Log2MaxMvLengthVertical:            11,
					MaxNumReorderFrames:                2,
					MaxDecFrameBuffering:               4,
				},
			},
			1920,
			1080,
			30,
		},
		{
			"1920x1080",
			[]byte{
				0x67, 0x64, 0x00, 0x29, 0xac, 0x13, 0x31, 0x40,
				0x78, 0x04, 0x47, 0xde, 0x03, 0xea, 0x02, 0x02,
				0x03, 0xe0, 0x00, 0x00, 0x03, 0x00, 0x20, 0x00,
				0x00, 0x06, 0x52, // 0x80,
			},
			SPS{
				ProfileIdc:                  100,
				LevelIdc:                    41,
				ChromeFormatIdc:             1,
				Log2MaxFrameNumMinus4:       8,
				Log2MaxPicOrderCntLsbMinus4: 5,
				MaxNumRefFrames:             4,
				PicWidthInMbsMinus1:         119,
				PicHeightInMbsMinus1:        33,
				Direct8x8InferenceFlag:      true,
				FrameCroppingFlag:           true,
				FrameCropBottomOffset:       2,
				VUI: &VUI{
					AspectRatioInfoPresentFlag:   true,
					AspectRatioIdc:               1,
					OverscanInfoPresentFlag:      true,
					OverscanAppropriateFlag:      true,
					VideoSignalTypePresentFlag:   true,
					VideoFormat:                  5,
					ColourDescriptionPresentFlag: true,
					ColourPrimaries:              1,
					TransferCharacteristics:      1,
					MatrixCoefficients:           1,
					ChromaLocInfoPresentFlag:     true,
					TimingInfoPresentFlag:        true,
					NumUnitsInTick:               1,
					TimeScale:                    50,
					FixedFrameRateFlag:           true,
					PicStructPresentFlag:         true,
				},
			},
			1920,
			1084,
			25,
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
