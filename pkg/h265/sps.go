package h265

import (
	"fmt"

	"github.com/aler9/gortsplib/v2/pkg/bits"
	"github.com/aler9/gortsplib/v2/pkg/h264"
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

// SPS_ProfileLevelTier is a profile level tier of a SPS.
type SPS_ProfileLevelTier struct { //nolint:revive
	GeneralProfileSpace             uint8
	GeneralTierFlag                 uint8
	GeneralProfileIdc               uint8
	GeneralProfileCompatibilityFlag [32]bool
	ProgressiveSourceFlag           bool
	InterlacedSourceFlag            bool
	NonPackedConstraintFlag         bool
	FrameOnlyConstraintFlag         bool
	Max12bitConstraintFlag          bool
	Max10bitConstraintFlag          bool
	Max8bitConstraintFlag           bool
	Max422ChromeConstraintFlag      bool
	Max420ChromaConstraintFlag      bool
	IntraConstraintFlag             bool
	OnePictureOnlyConstraintFlag    bool
	LowerBitRateConstraintFlag      bool
	Max14BitConstraintFlag          bool
	LevelIdc                        uint8
	SubLayerProfilePresentFlag      []bool
	SubLayerLevelPresentFlag        []bool
}

func (p *SPS_ProfileLevelTier) unmarshal(buf []byte, pos *int, maxNumSubLayersMinus1 uint8) error {
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

	p.ProgressiveSourceFlag = bits.ReadFlagUnsafe(buf, pos)
	p.InterlacedSourceFlag = bits.ReadFlagUnsafe(buf, pos)
	p.NonPackedConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.FrameOnlyConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.Max12bitConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.Max10bitConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.Max8bitConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.Max422ChromeConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.Max420ChromaConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.IntraConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.OnePictureOnlyConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
	p.LowerBitRateConstraintFlag = bits.ReadFlagUnsafe(buf, pos)

	if p.GeneralProfileIdc == 5 ||
		p.GeneralProfileIdc == 9 ||
		p.GeneralProfileIdc == 10 ||
		p.GeneralProfileIdc == 11 ||
		p.GeneralProfileCompatibilityFlag[5] ||
		p.GeneralProfileCompatibilityFlag[9] ||
		p.GeneralProfileCompatibilityFlag[10] ||
		p.GeneralProfileCompatibilityFlag[11] {
		p.Max14BitConstraintFlag = bits.ReadFlagUnsafe(buf, pos)
		*pos += 33
	} else {
		*pos += 34
	}

	p.LevelIdc = uint8(bits.ReadBitsUnsafe(buf, pos, 8))

	if maxNumSubLayersMinus1 > 0 {
		p.SubLayerProfilePresentFlag = make([]bool, maxNumSubLayersMinus1)
		p.SubLayerLevelPresentFlag = make([]bool, maxNumSubLayersMinus1)

		err := bits.HasSpace(buf, *pos, int(2*maxNumSubLayersMinus1))
		if err != nil {
			return err
		}
	}

	for j := uint8(0); j < maxNumSubLayersMinus1; j++ {
		p.SubLayerProfilePresentFlag[j] = bits.ReadFlagUnsafe(buf, pos)
		p.SubLayerLevelPresentFlag[j] = bits.ReadFlagUnsafe(buf, pos)
	}

	if maxNumSubLayersMinus1 > 0 {
		err := bits.HasSpace(buf, *pos, int(8-maxNumSubLayersMinus1)*2)
		if err != nil {
			return err
		}

		*pos += int(8-maxNumSubLayersMinus1) * 2
	}

	for i := uint8(0); i < maxNumSubLayersMinus1; i++ {
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

// SPS is a H265 sequence parameter set.
type SPS struct {
	VPSID                   uint8
	MaxNumSubLayersMinus1   uint8
	TemporalIDNestingFlag   bool
	ProfileLevelTier        SPS_ProfileLevelTier
	ID                      uint8
	ChromaFormatIdc         uint32
	SeparateColourPlaneFlag bool
	PicWidthInLumaSamples   uint32
	PicHeightInLumaSamples  uint32

	ConformanceWindow *SPS_ConformanceWindow

	BitDepthLumaMinus8          uint32
	BitDepthChromaMinus8        uint32
	Log2MaxPicOrderCntLsbMinus4 uint32
}

// Unmarshal decodes a SPS from bytes.
func (s *SPS) Unmarshal(buf []byte) error {
	if len(buf) < 2 {
		return fmt.Errorf("not enough bits")
	}

	typ := NALUType((buf[0] >> 1) & 0b111111)

	if typ != NALUTypeSPS {
		return fmt.Errorf("not a SPS")
	}

	buf = buf[2:]
	buf = h264.EmulationPreventionRemove(buf)
	pos := 0

	err := bits.HasSpace(buf, pos, 8)
	if err != nil {
		return err
	}

	s.VPSID = uint8(bits.ReadBitsUnsafe(buf, &pos, 4))
	s.MaxNumSubLayersMinus1 = uint8(bits.ReadBitsUnsafe(buf, &pos, 3))
	s.TemporalIDNestingFlag = bits.ReadFlagUnsafe(buf, &pos)

	err = s.ProfileLevelTier.unmarshal(buf, &pos, s.MaxNumSubLayersMinus1)
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
