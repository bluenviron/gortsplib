package h265

import (
	"fmt"

	"github.com/aler9/gortsplib/v2/pkg/bits"
	"github.com/aler9/gortsplib/v2/pkg/codecs/h264"
)

// PPS is a H265 picture parameter set.
type PPS struct {
	ID                                uint32
	SPSID                             uint32
	DependentSliceSegmentsEnabledFlag bool
	OutputFlagPresentFlag             bool
	NumExtraSliceHeaderBits           uint8
}

// Unmarshal decodes a PPS.
func (p *PPS) Unmarshal(buf []byte) error {
	buf = h264.EmulationPreventionRemove(buf)

	if len(buf) < 2 {
		return fmt.Errorf("not enough bits")
	}

	buf = buf[2:]
	pos := 0

	var err error
	p.ID, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	p.SPSID, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return err
	}

	p.DependentSliceSegmentsEnabledFlag, err = bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	p.OutputFlagPresentFlag, err = bits.ReadFlag(buf, &pos)
	if err != nil {
		return err
	}

	tmp, err := bits.ReadBits(buf, &pos, 3)
	if err != nil {
		return err
	}
	p.NumExtraSliceHeaderBits = uint8(tmp)

	return nil
}
