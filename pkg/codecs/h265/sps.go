package h265

import (
	"fmt"

	"github.com/aler9/gortsplib/v2/pkg/bits"
	"github.com/aler9/gortsplib/v2/pkg/codecs/h264"
)

const (
	maxNegativePics     = 255
	maxPositivePics     = 255
	maxShortTermRefPics = 64
)

var subWidthC = []uint32{
	1,
	2,
	2,
	1,
}

var subHeightC = []uint32{
	1,
	2,
	1,
	1,
}

// SPS_DefaultDisplayWindow is a default display window.
type SPS_DefaultDisplayWindow struct { //nolint:revive
	LeftOffset   uint32
	RightOffset  uint32
	TopOffset    uint32
	BottomOffset uint32
}

func (w *SPS_DefaultDisplayWindow) unmarshal(buf []byte, pos *int) error {
	var err error
	w.LeftOffset, err = bits.ReadGolombUnsigned(buf, pos)
	if err != nil {
		return err
	}

	w.RightOffset, err = bits.ReadGolombUnsigned(buf, pos)
	if err != nil {
		return err
	}

	w.TopOffset, err = bits.ReadGolombUnsigned(buf, pos)
	if err != nil {
		return err
	}

	w.BottomOffset, err = bits.ReadGolombUnsigned(buf, pos)
	if err != nil {
		return err
	}

	return nil
}

// SPS_TimingInfo is a timing info.
type SPS_TimingInfo struct { //nolint:revive
	NumUnitsInTick              uint32
	TimeScale                   uint32
	POCProportionalToTimingFlag bool

	// POCProportionalToTimingFlag == true
	NumTicksPOCDiffOneMinus1 uint32
}

func (t *SPS_TimingInfo) unmarshal(buf []byte, pos *int) error {
	err := bits.HasSpace(buf, *pos, 32+32+1)
	if err != nil {
		return err
	}

	t.NumUnitsInTick = uint32(bits.ReadBitsUnsafe(buf, pos, 32))
	t.TimeScale = uint32(bits.ReadBitsUnsafe(buf, pos, 32))
	t.POCProportionalToTimingFlag = bits.ReadFlagUnsafe(buf, pos)

	if t.POCProportionalToTimingFlag {
		t.NumTicksPOCDiffOneMinus1, err = bits.ReadGolombUnsigned(buf, pos)
		if err != nil {
			return err
		}
	}

	return nil
}

// SPS_VUI is a video usability information.
type SPS_VUI struct { //nolint:revive
	AspectRatioInfoPresentFlag bool

	// AspectRatioInfoPresentFlag == true
	AspectRatioIdc uint8
	SarWidth       uint16
	SarHeight      uint16

	OverscanInfoPresentFlag bool

	// OverscanInfoPresentFlag == true
	OverscanAppropriateFlag    bool
	VideoSignalTypePresentFlag bool

	// VideoSignalTypePresentFlag == true
	VideoFormat                  uint8
	VideoFullRangeFlag           bool
	ColourDescriptionPresentFlag bool

	// ColourDescriptionPresentFlag == true
	ColourPrimaries         uint8
	TransferCharacteristics uint8
	MatrixCoefficients      uint8

	ChromaLocInfoPresentFlag bool

	// ChromaLocInfoPresentFlag == true
	ChromaSampleLocTypeTopField    uint32
	ChromaSampleLocTypeBottomField uint32

	NeutralChromaIndicationFlag bool
	FieldSeqFlag                bool
	FrameFieldInfoPresentFlag   bool
	DefaultDisplayWindow        *SPS_DefaultDisplayWindow
	TimingInfo                  *SPS_TimingInfo
}

func (v *SPS_VUI) unmarshal(buf []byte, pos *int) error {
	var err error
	v.AspectRatioInfoPresentFlag, err = bits.ReadFlag(buf, pos)
	if err != nil {
		return err
	}

	if v.AspectRatioInfoPresentFlag {
		tmp, err := bits.ReadBits(buf, pos, 8)
		if err != nil {
			return err
		}
		v.AspectRatioIdc = uint8(tmp)

		if v.AspectRatioIdc == 255 { // EXTENDED_SAR
			err := bits.HasSpace(buf, *pos, 32)
			if err != nil {
				return err
			}

			v.SarWidth = uint16(bits.ReadBitsUnsafe(buf, pos, 16))
			v.SarHeight = uint16(bits.ReadBitsUnsafe(buf, pos, 16))
		}
	}

	v.OverscanInfoPresentFlag, err = bits.ReadFlag(buf, pos)
	if err != nil {
		return err
	}

	if v.OverscanInfoPresentFlag {
		v.OverscanAppropriateFlag, err = bits.ReadFlag(buf, pos)
		if err != nil {
			return err
		}
	}

	v.VideoSignalTypePresentFlag, err = bits.ReadFlag(buf, pos)
	if err != nil {
		return err
	}

	if v.VideoSignalTypePresentFlag {
		err := bits.HasSpace(buf, *pos, 5)
		if err != nil {
			return err
		}

		v.VideoFormat = uint8(bits.ReadBitsUnsafe(buf, pos, 3))
		v.VideoFullRangeFlag = bits.ReadFlagUnsafe(buf, pos)
		v.ColourDescriptionPresentFlag = bits.ReadFlagUnsafe(buf, pos)

		if v.ColourDescriptionPresentFlag {
			err := bits.HasSpace(buf, *pos, 24)
			if err != nil {
				return err
			}

			v.ColourPrimaries = uint8(bits.ReadBitsUnsafe(buf, pos, 8))
			v.TransferCharacteristics = uint8(bits.ReadBitsUnsafe(buf, pos, 8))
			v.MatrixCoefficients = uint8(bits.ReadBitsUnsafe(buf, pos, 8))
		}
	}

	v.ChromaLocInfoPresentFlag, err = bits.ReadFlag(buf, pos)
	if err != nil {
		return err
	}

	if v.ChromaLocInfoPresentFlag {
		v.ChromaSampleLocTypeTopField, err = bits.ReadGolombUnsigned(buf, pos)
		if err != nil {
			return err
		}

		v.ChromaSampleLocTypeBottomField, err = bits.ReadGolombUnsigned(buf, pos)
		if err != nil {
			return err
		}
	}

	v.NeutralChromaIndicationFlag, err = bits.ReadFlag(buf, pos)
	if err != nil {
		return err
	}

	v.FieldSeqFlag, err = bits.ReadFlag(buf, pos)
	if err != nil {
		return err
	}

	v.FrameFieldInfoPresentFlag, err = bits.ReadFlag(buf, pos)
	if err != nil {
		return err
	}

	defaultDisplayWindowFlag, err := bits.ReadFlag(buf, pos)
	if err != nil {
		return err
	}

	if defaultDisplayWindowFlag {
		v.DefaultDisplayWindow = &SPS_DefaultDisplayWindow{}
		err := v.DefaultDisplayWindow.unmarshal(buf, pos)
		if err != nil {
			return err
		}
	} else {
		v.DefaultDisplayWindow = nil
	}

	timingInfoPresentFlag, err := bits.ReadFlag(buf, pos)
	if err != nil {
		return err
	}

	if timingInfoPresentFlag {
		v.TimingInfo = &SPS_TimingInfo{}
		err := v.TimingInfo.unmarshal(buf, pos)
		if err != nil {
			return err
		}
	} else {
		v.TimingInfo = nil
	}

	return nil
}

// SPS_ProfileTierLevel is a profile level tier of a SPS.
type SPS_ProfileTierLevel struct { //nolint:revive
	GeneralProfileSpace                 uint8
	GeneralTierFlag                     uint8
	GeneralProfileIdc                   uint8
	GeneralProfileCompatibilityFlag     [32]bool
	GeneralProgressiveSourceFlag        bool
	GeneralInterlacedSourceFlag         bool
	GeneralNonPackedConstraintFlag      bool
	GeneralFrameOnlyConstraintFlag      bool
	GeneralMax12bitConstraintFlag       bool
	GeneralMax10bitConstraintFlag       bool
	GeneralMax8bitConstraintFlag        bool
	GeneralMax422ChromeConstraintFlag   bool
	GeneralMax420ChromaConstraintFlag   bool
	GeneralMaxMonochromeConstraintFlag  bool
	GeneralIntraConstraintFlag          bool
	GeneralOnePictureOnlyConstraintFlag bool
	GeneralLowerBitRateConstraintFlag   bool
	GeneralMax14BitConstraintFlag       bool
	GeneralLevelIdc                     uint8
	SubLayerProfilePresentFlag          []bool
	SubLayerLevelPresentFlag            []bool
}

func (p *SPS_ProfileTierLevel) unmarshal(buf []byte, pos *int, maxSubLayersMinus1 uint8) error {
	err := bits.HasSpace(buf, *pos, 8+32+12+34+8)
	if err != nil {
		return err
	}

	p.GeneralProfileSpace = uint8(bits.ReadBitsUnsafe(buf, pos, 2))
	p.GeneralTierFlag = uint8(bits.ReadBitsUnsafe(buf, pos, 1))
	p.GeneralProfileIdc = uint8(bits.ReadBitsUnsafe(buf, pos, 5))

	for j := 0; j < 32; j++ {
		p.GeneralProfileCompatibilityFlag[j] = bits.ReadFlagUnsafe(buf, pos)
	}

	p.GeneralProgressiveSourceFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralInterlacedSourceFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralNonPackedConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralFrameOnlyConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralMax12bitConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralMax10bitConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralMax8bitConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralMax422ChromeConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralMax420ChromaConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralMaxMonochromeConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralIntraConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralOnePictureOnlyConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.GeneralLowerBitRateConstraintFlag = bits.ReadFlagUnsafe(buf, pos)

	if p.GeneralProfileIdc == 5 ||
		p.GeneralProfileIdc == 9 ||
		p.GeneralProfileIdc == 10 ||
		p.GeneralProfileIdc == 11 ||
		p.GeneralProfileCompatibilityFlag[5] ||
		p.GeneralProfileCompatibilityFlag[9] ||
		p.GeneralProfileCompatibilityFlag[10] ||
		p.GeneralProfileCompatibilityFlag[11] {
		p.GeneralMax14BitConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
		*pos += 34
	} else {
		*pos += 35
	}

	p.GeneralLevelIdc = uint8(bits.ReadBitsUnsafe(buf, pos, 8))

	if maxSubLayersMinus1 > 0 {
		p.SubLayerProfilePresentFlag = make([]bool, maxSubLayersMinus1)
		p.SubLayerLevelPresentFlag = make([]bool, maxSubLayersMinus1)

		err := bits.HasSpace(buf, *pos, int(2*maxSubLayersMinus1))
		if err != nil {
			return err
		}

		for j := uint8(0); j < maxSubLayersMinus1; j++ {
			p.SubLayerProfilePresentFlag[j] = bits.ReadFlagUnsafe(buf, pos)
			p.SubLayerLevelPresentFlag[j] = bits.ReadFlagUnsafe(buf, pos)
		}
	}

	if maxSubLayersMinus1 > 0 {
		err := bits.HasSpace(buf, *pos, int(8-maxSubLayersMinus1)*2)
		if err != nil {
			return err
		}

		*pos += int(8-maxSubLayersMinus1) * 2
	}

	for i := uint8(0); i < maxSubLayersMinus1; i++ {
		if p.SubLayerProfilePresentFlag[i] {
			return fmt.Errorf("SubLayerProfilePresentFlag not supported yet")
		}

		if p.SubLayerLevelPresentFlag[i] {
			return fmt.Errorf("SubLayerLevelPresentFlag not supported yet")
		}
	}

	return nil
}

// SPS_ConformanceWindow is a conformance window of a SPS.
type SPS_ConformanceWindow struct { //nolint:revive
	LeftOffset   uint32
	RightOffset  uint32
	TopOffset    uint32
	BottomOffset uint32
}

func (c *SPS_ConformanceWindow) unmarshal(buf []byte, pos *int) error {
	var err error
	c.LeftOffset, err = bits.ReadGolombUnsigned(buf, pos)
	if err != nil {
		return err
	}

	c.RightOffset, err = bits.ReadGolombUnsigned(buf, pos)
	if err != nil {
		return err
	}

	c.TopOffset, err = bits.ReadGolombUnsigned(buf, pos)
	if err != nil {
		return err
	}

	c.BottomOffset, err = bits.ReadGolombUnsigned(buf, pos)
	if err != nil {
		return err
	}

	return nil
}

// SPS_ShortTermRefPicSet is a short-term reference picture set.
type SPS_ShortTermRefPicSet struct { //nolint:revive
	InterRefPicSetPredictionFlag bool
	DeltaIdxMinus1               uint32
	DeltaRpsSign                 bool
	AbsDeltaRpsMinus1            uint32
	NumNegativePics              uint32
	NumPositivePics              uint32
	DeltaPocS0Minus1             []uint32
	UsedByCurrPicS0Flag          []bool
	DeltaPocS1Minus1             []uint32
	UsedByCurrPicS1Flag          []bool
}

func (r *SPS_ShortTermRefPicSet) unmarshal(buf []byte, pos *int, stRpsIdx uint32,
	numShortTermRefPicSets uint32, shortTermRefPicSets []*SPS_ShortTermRefPicSet,
) error {
	var err error

	if stRpsIdx != 0 {
		r.InterRefPicSetPredictionFlag, err = bits.ReadFlag(buf, pos)
		if err != nil {
			return err
		}
	}

	if r.InterRefPicSetPredictionFlag {
		if stRpsIdx == numShortTermRefPicSets {
			r.DeltaIdxMinus1, err = bits.ReadGolombUnsigned(buf, pos)
			if err != nil {
				return err
			}
		}

		r.DeltaRpsSign, err = bits.ReadFlag(buf, pos)
		if err != nil {
			return err
		}

		r.AbsDeltaRpsMinus1, err = bits.ReadGolombUnsigned(buf, pos)
		if err != nil {
			return err
		}

		refRpsIdx := stRpsIdx - (r.DeltaIdxMinus1 + 1)
		numDeltaPocs := shortTermRefPicSets[refRpsIdx].NumNegativePics + shortTermRefPicSets[refRpsIdx].NumPositivePics

		for j := uint32(0); j <= numDeltaPocs; j++ {
			usedByCurrPicFlag, err := bits.ReadFlag(buf, pos)
			if err != nil {
				return err
			}

			if usedByCurrPicFlag {
				_, err := bits.ReadGolombUnsigned(buf, pos) // use_delta_flag
				if err != nil {
					return err
				}
			}
		}
	} else {
		r.NumNegativePics, err = bits.ReadGolombUnsigned(buf, pos)
		if err != nil {
			return err
		}

		r.NumPositivePics, err = bits.ReadGolombUnsigned(buf, pos)
		if err != nil {
			return err
		}

		if r.NumNegativePics > 0 {
			if r.NumNegativePics > maxNegativePics {
				return fmt.Errorf("num_negative_pics exceeds %d", maxNegativePics)
			}

			r.DeltaPocS0Minus1 = make([]uint32, r.NumNegativePics)
			r.UsedByCurrPicS0Flag = make([]bool, r.NumNegativePics)

			for i := uint32(0); i < r.NumNegativePics; i++ {
				r.DeltaPocS0Minus1[i], err = bits.ReadGolombUnsigned(buf, pos)
				if err != nil {
					return err
				}

				r.UsedByCurrPicS0Flag[i], err = bits.ReadFlag(buf, pos)
				if err != nil {
					return err
				}
			}
		}

		if r.NumPositivePics > 0 {
			if r.NumPositivePics > maxPositivePics {
				return fmt.Errorf("num_positive_pics exceeds %d", maxPositivePics)
			}

			r.DeltaPocS1Minus1 = make([]uint32, r.NumPositivePics)
			r.UsedByCurrPicS1Flag = make([]bool, r.NumPositivePics)

			for i := uint32(0); i < r.NumPositivePics; i++ {
				r.DeltaPocS1Minus1[i], err = bits.ReadGolombUnsigned(buf, pos)
				if err != nil {
					return err
				}

				r.UsedByCurrPicS1Flag[i], err = bits.ReadFlag(buf, pos)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// SPS is a H265 sequence parameter set.
type SPS struct {
	VPSID                                uint8
	MaxSubLayersMinus1                   uint8
	TemporalIDNestingFlag                bool
	ProfileTierLevel                     SPS_ProfileTierLevel
	ID                                   uint8
	ChromaFormatIdc                      uint32
	SeparateColourPlaneFlag              bool
	PicWidthInLumaSamples                uint32
	PicHeightInLumaSamples               uint32
	ConformanceWindow                    *SPS_ConformanceWindow
	BitDepthLumaMinus8                   uint32
	BitDepthChromaMinus8                 uint32
	Log2MaxPicOrderCntLsbMinus4          uint32
	SubLayerOrderingInfoPresentFlag      bool
	MaxDecPicBufferingMinus1             []uint32
	MaxNumReorderPics                    []uint32
	MaxLatencyIncreasePlus1              []uint32
	Log2MinLumaCodingBlockSizeMinus3     uint32
	Log2DiffMaxMinLumaCodingBlockSize    uint32
	Log2MinLumaTransformBlockSizeMinus2  uint32
	Log2DiffMaxMinLumaTransformBlockSize uint32
	MaxTransformHierarchyDepthInter      uint32
	MaxTransformHierarchyDepthIntra      uint32
	ScalingListEnabledFlag               bool
	ScalingListDataPresentFlag           bool
	AmpEnabledFlag                       bool
	SampleAdaptiveOffsetEnabledFlag      bool
	PcmEnabledFlag                       bool

	// PcmEnabledFlag == true
	PcmSampleBitDepthLumaMinus1          uint8
	PcmSampleBitDepthChromaMinus1        uint8
	Log2MinPcmLumaCodingBlockSizeMinus3  uint32
	Log2DiffMaxMinPcmLumaCodingBlockSize uint32
	PcmLoopFilterDisabledFlag            bool

	ShortTermRefPicSets             []*SPS_ShortTermRefPicSet
	LongTermRefPicsPresentFlag      bool
	TemporalMvpEnabledFlag          bool
	StrongIntraSmoothingEnabledFlag bool
	VUI                             *SPS_VUI
}

// Unmarshal decodes a SPS from bytes.
func (s *SPS) Unmarshal(buf []byte) error {
	buf = h264.EmulationPreventionRemove(buf)

	if len(buf) < 2 {
		return fmt.Errorf("not enough bits")
	}

	buf = buf[2:]
	pos := 0

	err := bits.HasSpace(buf, pos, 8)
	if err != nil {
		return err
	}

	s.VPSID = uint8(bits.ReadBitsUnsafe(buf, &pos, 4))
	s.MaxSubLayersMinus1 = uint8(bits.ReadBitsUnsafe(buf, &pos, 3))
	s.TemporalIDNestingFlag = bits.ReadFlagUnsafe(buf, &pos)

	err = s.ProfileTierLevel.unmarshal(buf, &pos, s.MaxSubLayersMinus1)
	if err != nil {
		return err
	}

	tmp2, err := bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}
	s.ID = uint8(tmp2)

	s.ChromaFormatIdc, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	if s.ChromaFormatIdc == 3 {
		s.SeparateColourPlaneFlag, err = bits.ReadFlag(buf, &pos)
		if err != nil {
			return err
		}
	}

	s.PicWidthInLumaSamples, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	s.PicHeightInLumaSamples, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	conformanceWindowFlag, err := bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	if conformanceWindowFlag {
		s.ConformanceWindow = &SPS_ConformanceWindow{}
		err := s.ConformanceWindow.unmarshal(buf, &pos)
		if err != nil {
			return err
		}
	} else {
		s.ConformanceWindow = nil
	}

	s.BitDepthLumaMinus8, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	s.BitDepthChromaMinus8, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	s.Log2MaxPicOrderCntLsbMinus4, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	s.SubLayerOrderingInfoPresentFlag, err = bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	var start uint8
	if s.SubLayerOrderingInfoPresentFlag {
		start = 0
	} else {
		start = s.MaxSubLayersMinus1
	}

	s.MaxDecPicBufferingMinus1 = make([]uint32, s.MaxSubLayersMinus1+1)
	s.MaxNumReorderPics = make([]uint32, s.MaxSubLayersMinus1+1)
	s.MaxLatencyIncreasePlus1 = make([]uint32, s.MaxSubLayersMinus1+1)

	for i := start; i <= s.MaxSubLayersMinus1; i++ {
		s.MaxDecPicBufferingMinus1[i], err = bits.ReadGolombUnsigned(buf, &pos)
		if err != nil {
			return err
		}

		s.MaxNumReorderPics[i], err = bits.ReadGolombUnsigned(buf, &pos)
		if err != nil {
			return err
		}

		s.MaxLatencyIncreasePlus1[i], err = bits.ReadGolombUnsigned(buf, &pos)
		if err != nil {
			return err
		}
	}

	s.Log2MinLumaCodingBlockSizeMinus3, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	s.Log2DiffMaxMinLumaCodingBlockSize, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	s.Log2MinLumaTransformBlockSizeMinus2, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	s.Log2DiffMaxMinLumaTransformBlockSize, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	s.MaxTransformHierarchyDepthInter, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	s.MaxTransformHierarchyDepthIntra, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	s.ScalingListEnabledFlag, err = bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	if s.ScalingListEnabledFlag {
		s.ScalingListDataPresentFlag, err = bits.ReadFlag(buf, &pos)
		if err != nil {
			return err
		}

		if s.ScalingListDataPresentFlag {
			return fmt.Errorf("ScalingListDataPresentFlag not supported yet")
		}
	}

	s.AmpEnabledFlag, err = bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	s.SampleAdaptiveOffsetEnabledFlag, err = bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	s.PcmEnabledFlag, err = bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	if s.PcmEnabledFlag {
		err := bits.HasSpace(buf, pos, 8)
		if err != nil {
			return err
		}

		s.PcmSampleBitDepthLumaMinus1 = uint8(bits.ReadBitsUnsafe(buf, &pos, 4))
		s.PcmSampleBitDepthChromaMinus1 = uint8(bits.ReadBitsUnsafe(buf, &pos, 4))

		s.Log2MinPcmLumaCodingBlockSizeMinus3, err = bits.ReadGolombUnsigned(buf, &pos)
		if err != nil {
			return err
		}

		s.Log2DiffMaxMinPcmLumaCodingBlockSize, err = bits.ReadGolombUnsigned(buf, &pos)
		if err != nil {
			return err
		}

		s.PcmLoopFilterDisabledFlag, err = bits.ReadFlag(buf, &pos)
		if err != nil {
			return err
		}
	}

	numShortTermRefPicSets, err := bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	if numShortTermRefPicSets > 0 {
		if numShortTermRefPicSets > maxShortTermRefPics {
			return fmt.Errorf("num_short_term_ref_pic_sets exceeds %d", maxShortTermRefPics)
		}

		s.ShortTermRefPicSets = make([]*SPS_ShortTermRefPicSet, numShortTermRefPicSets)

		for i := uint32(0); i < numShortTermRefPicSets; i++ {
			s.ShortTermRefPicSets[i] = &SPS_ShortTermRefPicSet{}
			err := s.ShortTermRefPicSets[i].unmarshal(buf, &pos, i, numShortTermRefPicSets, s.ShortTermRefPicSets)
			if err != nil {
				return err
			}
		}
	} else {
		s.ShortTermRefPicSets = nil
	}

	s.LongTermRefPicsPresentFlag, err = bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	if s.LongTermRefPicsPresentFlag {
		return fmt.Errorf("LongTermRefPicsPresentFlag not supported yet")
	}

	s.TemporalMvpEnabledFlag, err = bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	s.StrongIntraSmoothingEnabledFlag, err = bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	vuiParametersPresentFlag, err := bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	if vuiParametersPresentFlag {
		s.VUI = &SPS_VUI{}
		err := s.VUI.unmarshal(buf, &pos)
		if err != nil {
			return err
		}
	} else {
		s.VUI = nil
	}

	return nil
}

// Width returns the video width.
func (s SPS) Width() int {
	width := s.PicWidthInLumaSamples

	if s.ConformanceWindow != nil {
		cropUnitX := subWidthC[s.ChromaFormatIdc]
		width -= (s.ConformanceWindow.LeftOffset + s.ConformanceWindow.RightOffset) * cropUnitX
	}

	return int(width)
}

// Height returns the video height.
func (s SPS) Height() int {
	height := s.PicHeightInLumaSamples

	if s.ConformanceWindow != nil {
		cropUnitY := subHeightC[s.ChromaFormatIdc]
		height -= (s.ConformanceWindow.TopOffset + s.ConformanceWindow.BottomOffset) * cropUnitY
	}

	return int(height)
}

// FPS returns the frames per second of the video.
func (s SPS) FPS() float64 {
	if s.VUI == nil || s.VUI.TimingInfo == nil {
		return 0
	}

	return float64(s.VUI.TimingInfo.TimeScale) / float64(s.VUI.TimingInfo.NumUnitsInTick)
}
