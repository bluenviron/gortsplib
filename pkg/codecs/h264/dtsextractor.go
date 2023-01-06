package h264

import (
	"fmt"
	"time"

	"github.com/aler9/gortsplib/v2/pkg/bits"
)

func getPictureOrderCount(buf []byte, sps *SPS) (uint32, error) {
	buf = EmulationPreventionRemove(buf[:6])

	buf = buf[1:]
	pos := 0

	_, err := bits.ReadGolombUnsigned(buf, &pos) // first_mb_in_slice
	if err != nil {
		return 0, err
	}

	_, err = bits.ReadGolombUnsigned(buf, &pos) // slice_type
	if err != nil {
		return 0, err
	}

	_, err = bits.ReadGolombUnsigned(buf, &pos) // pic_parameter_set_id
	if err != nil {
		return 0, err
	}

	_, err = bits.ReadBits(buf, &pos, int(sps.Log2MaxFrameNumMinus4+4)) // frame_num
	if err != nil {
		return 0, err
	}

	if !sps.FrameMbsOnlyFlag {
		return 0, fmt.Errorf("frame_mbs_only_flag = 0 is not supported")
	}

	picOrderCntLsb, err := bits.ReadBits(buf, &pos, int(sps.Log2MaxPicOrderCntLsbMinus4+4))
	if err != nil {
		return 0, err
	}

	return uint32(picOrderCntLsb), nil
}

func findPictureOrderCount(au [][]byte, sps *SPS) (uint32, error) {
	for _, nalu := range au {
		typ := NALUType(nalu[0] & 0x1F)
		if typ == NALUTypeNonIDR {
			poc, err := getPictureOrderCount(nalu, sps)
			if err != nil {
				return 0, err
			}
			return poc, nil
		}
	}
	return 0, fmt.Errorf("POC not found")
}

func getPictureOrderCountDiff(poc1 uint32, poc2 uint32, sps *SPS) int32 {
	diff := int32(poc1) - int32(poc2)
	switch {
	case diff < -((1 << (sps.Log2MaxPicOrderCntLsbMinus4 + 3)) - 1):
		diff += (1 << (sps.Log2MaxPicOrderCntLsbMinus4 + 4))

	case diff > ((1 << (sps.Log2MaxPicOrderCntLsbMinus4 + 3)) - 1):
		diff -= (1 << (sps.Log2MaxPicOrderCntLsbMinus4 + 4))
	}
	return diff
}

// DTSExtractor allows to extract DTS from PTS.
type DTSExtractor struct {
	spsp            *SPS
	prevDTSFilled   bool
	prevDTS         time.Duration
	expectedPOC     uint32
	reorderedFrames int
	pauseDTS        int
}

// NewDTSExtractor allocates a DTSExtractor.
func NewDTSExtractor() *DTSExtractor {
	return &DTSExtractor{}
}

func (d *DTSExtractor) extractInner(au [][]byte, pts time.Duration) (time.Duration, error) {
	idrPresent := false

	for _, nalu := range au {
		typ := NALUType(nalu[0] & 0x1F)
		switch typ {
		case NALUTypeSPS:
			var spsp SPS
			err := spsp.Unmarshal(nalu)
			if err != nil {
				return 0, fmt.Errorf("invalid SPS: %v", err)
			}
			d.spsp = &spsp

		case NALUTypeIDR:
			idrPresent = true
		}
	}

	if d.spsp == nil {
		return 0, fmt.Errorf("SPS not received yet")
	}

	if d.spsp.PicOrderCntType == 2 {
		return pts, nil
	}

	if d.spsp.PicOrderCntType == 1 {
		return 0, fmt.Errorf("pic_order_cnt_type = 1 is not supported yet")
	}

	if idrPresent {
		d.expectedPOC = 0
		d.reorderedFrames = 0
		d.pauseDTS = 0
		return pts, nil
	}

	d.expectedPOC += 2
	d.expectedPOC &= ((1 << (d.spsp.Log2MaxPicOrderCntLsbMinus4 + 4)) - 1)

	if d.pauseDTS > 0 {
		d.pauseDTS--
		return d.prevDTS + 1*time.Millisecond, nil
	}

	poc, err := findPictureOrderCount(au, d.spsp)
	if err != nil {
		return 0, err
	}

	pocDiff := int(getPictureOrderCountDiff(poc, d.expectedPOC, d.spsp)) + d.reorderedFrames*2

	if pocDiff < 0 {
		return 0, fmt.Errorf("invalid POC")
	}

	if pocDiff == 0 {
		return pts, nil
	}

	reorderedFrames := (pocDiff - d.reorderedFrames*2) / 2
	if reorderedFrames > d.reorderedFrames {
		d.pauseDTS = (reorderedFrames - d.reorderedFrames - 1)
		d.reorderedFrames = reorderedFrames
		return d.prevDTS + 1*time.Millisecond, nil
	}

	return d.prevDTS + ((pts - d.prevDTS) * 2 / time.Duration(pocDiff+2)), nil
}

// Extract extracts the DTS of a access unit.
func (d *DTSExtractor) Extract(au [][]byte, pts time.Duration) (time.Duration, error) {
	dts, err := d.extractInner(au, pts)
	if err != nil {
		return 0, err
	}

	if dts > pts {
		return 0, fmt.Errorf("DTS is greater than PTS")
	}

	if d.prevDTSFilled && dts <= d.prevDTS {
		return 0, fmt.Errorf("DTS is not monotonically increasing, was %v, now is %v",
			d.prevDTS, dts)
	}

	d.prevDTS = dts
	d.prevDTSFilled = true

	return dts, err
}
