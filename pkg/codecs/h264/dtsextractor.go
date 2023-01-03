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

	var picOrderCntLsb uint64
	switch {
	case sps.PicOrderCntType == 0:
		picOrderCntLsb, err = bits.ReadBits(buf, &pos, int(sps.Log2MaxPicOrderCntLsbMinus4+4))
		if err != nil {
			return 0, err
		}

	default:
		return 0, fmt.Errorf("pic_order_cnt_type = 1 is not supported")
	}

	return uint32(picOrderCntLsb), nil
}

func findPictureOrderCount(nalus [][]byte, sps *SPS) (uint32, error) {
	for _, nalu := range nalus {
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

type seiTimingInfo struct {
	cpbRemovalDelay uint32
	dpbOutputDelay  uint32
}

func parseSEITimingInfo(buf []byte, sps *SPS) (*seiTimingInfo, bool) {
	buf = EmulationPreventionRemove(buf)
	pos := 1

	for {
		if pos >= (len(buf) - 1) {
			return nil, false
		}

		payloadType := 0
		for {
			byt := buf[pos]
			pos++
			payloadType += int(byt)
			if byt != 0xFF {
				break
			}
		}

		payloadSize := 0
		for {
			byt := buf[pos]
			pos++
			payloadSize += int(byt)
			if byt != 0xFF {
				break
			}
		}

		if payloadType == 1 { // timing info
			buf2 := buf[pos:]
			pos2 := 0

			ret := &seiTimingInfo{}

			tmp, err := bits.ReadBits(buf2, &pos2, int(sps.VUI.NalHRD.CpbRemovalDelayLengthMinus1+1))
			if err != nil {
				return nil, false
			}
			ret.cpbRemovalDelay = uint32(tmp)

			tmp, err = bits.ReadBits(buf2, &pos2, int(sps.VUI.NalHRD.DpbOutputDelayLengthMinus1+1))
			if err != nil {
				return nil, false
			}
			ret.dpbOutputDelay = uint32(tmp)

			return ret, true
		}

		pos += payloadSize
	}
}

func findSEITimingInfo(nalus [][]byte, sps *SPS) (*seiTimingInfo, bool) {
	for _, nalu := range nalus {
		typ := NALUType(nalu[0] & 0x1F)
		if typ == NALUTypeSEI {
			ret, ok := parseSEITimingInfo(nalu, sps)
			if ok {
				return ret, true
			}
		}
	}
	return nil, false
}

// DTSExtractor allows to extract DTS from PTS.
type DTSExtractor struct {
	spsp          *SPS
	prevPTS       time.Duration
	prevDTSFilled bool
	prevDTS       time.Duration
	expectedPOC   uint32
}

// NewDTSExtractor allocates a DTSExtractor.
func NewDTSExtractor() *DTSExtractor {
	return &DTSExtractor{}
}

// returns the difference between PTS POC (picture order count) and DTS POC.
func (d *DTSExtractor) findPOCDiff(idrPresent bool, nalus [][]byte) (int, error) {
	switch {
	// POC difference is computed by using PTS POC, timing infos and max_num_reorder_frames
	case d.spsp.PicOrderCntType != 2 &&
		d.spsp.VUI != nil && d.spsp.VUI.TimingInfo != nil && d.spsp.VUI.BitstreamRestriction != nil:
		if idrPresent {
			d.expectedPOC = 0
			return int(d.spsp.VUI.BitstreamRestriction.MaxNumReorderFrames * 2), nil
		}

		// compute expectedPOC immediately in order to store it even in case of errors
		d.expectedPOC += 2
		d.expectedPOC &= ((1 << (d.spsp.Log2MaxPicOrderCntLsbMinus4 + 4)) - 1)

		poc, err := findPictureOrderCount(nalus, d.spsp)
		if err != nil {
			return 0, err
		}

		pocDiff := int(getPictureOrderCountDiff(poc, d.expectedPOC, d.spsp)) +
			int(d.spsp.VUI.BitstreamRestriction.MaxNumReorderFrames*2)

		return pocDiff, nil

	// POC difference is computed from SEI
	case d.spsp.VUI != nil && d.spsp.VUI.TimingInfo != nil && d.spsp.VUI.NalHRD != nil:
		ti, ok := findSEITimingInfo(nalus, d.spsp)
		if !ok {
			// some streams declare that they use SEI pic timings, but they don't.
			// assume PTS = DTS.
			return 0, nil
		}

		// workaround for nvenc
		// nvenc puts a wrong dpbOutputDelay into timing infos of non-starting IDR frames
		// https://forums.developer.nvidia.com/t/nvcodec-h-264-encoder-sei-pic-timing-dpb-output-delay/156050
		// https://forums.developer.nvidia.com/t/h264-pic-timing-sei-message/71188
		if idrPresent && ti.cpbRemovalDelay > 0 {
			ti.dpbOutputDelay = 2
		}

		return int(ti.dpbOutputDelay), nil

	// assume PTS = DTS
	default:
		return 0, nil
	}
}

func (d *DTSExtractor) extractInner(nalus [][]byte, pts time.Duration) (time.Duration, error) {
	idrPresent := false

	for _, nalu := range nalus {
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

	pocDiff, err := d.findPOCDiff(idrPresent, nalus)
	if err != nil {
		return 0, err
	}

	timeDiff := time.Duration(pocDiff) * time.Second *
		time.Duration(d.spsp.VUI.TimingInfo.NumUnitsInTick) / time.Duration(d.spsp.VUI.TimingInfo.TimeScale)
	dts := pts - timeDiff

	return dts, nil
}

// Extract extracts the DTS of a group of NALUs.
func (d *DTSExtractor) Extract(nalus [][]byte, pts time.Duration) (time.Duration, error) {
	dts, err := d.extractInner(nalus, pts)
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

	d.prevPTS = pts
	d.prevDTS = dts
	d.prevDTSFilled = true
	return dts, err
}
