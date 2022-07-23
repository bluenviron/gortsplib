package h264

import (
	"bytes"
	"fmt"
	"math"
	"time"

	"github.com/aler9/gortsplib/pkg/bits"
)

func getPOC(buf []byte, sps *SPS) (uint32, error) {
	buf = AntiCompetitionRemove(buf[:6])

	isIDR := NALUType(buf[0]&0x1F) == NALUTypeIDR

	buf = buf[1:]
	pos := 0

	// first_mb_in_slice
	_, err := bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return 0, err
	}

	// slice_type
	_, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return 0, err
	}

	// pic_parameter_set_id
	_, err = bits.ReadGolombUnsigned(buf, &pos)
	if err != nil {
		return 0, err
	}

	// frame_num
	_, err = bits.ReadBits(buf, &pos, int(sps.Log2MaxFrameNumMinus4+4))
	if err != nil {
		return 0, err
	}

	if !sps.FrameMbsOnlyFlag {
		return 0, fmt.Errorf("unsupported")
	}

	if isIDR {
		// idr_pic_id
		_, err = bits.ReadGolombUnsigned(buf, &pos)
		if err != nil {
			return 0, err
		}
	}

	var picOrderCntLsb uint64
	switch {
	case sps.PicOrderCntType == 0:
		picOrderCntLsb, err = bits.ReadBits(buf, &pos, int(sps.Log2MaxPicOrderCntLsbMinus4+4))
		if err != nil {
			return 0, err
		}

	default:
		return 0, fmt.Errorf("pic_order_cnt_type = 1 is unsupported")
	}

	return uint32(picOrderCntLsb), nil
}

func findPOC(nalus [][]byte, sps *SPS) (uint32, error) {
	for _, nalu := range nalus {
		typ := NALUType(nalu[0] & 0x1F)
		if typ == NALUTypeIDR || typ == NALUTypeNonIDR {
			poc, err := getPOC(nalu, sps)
			if err != nil {
				return 0, err
			}
			return poc, nil
		}
	}
	return 0, fmt.Errorf("POC not found")
}

func getPOCDiff(poc1 uint32, poc2 uint32, sps *SPS) int32 {
	diff := int32(poc1) - int32(poc2)
	switch {
	case diff < -((1 << (sps.Log2MaxPicOrderCntLsbMinus4 + 3)) - 1):
		diff += (1 << (sps.Log2MaxPicOrderCntLsbMinus4 + 4))

	case diff > ((1 << (sps.Log2MaxPicOrderCntLsbMinus4 + 3)) - 1):
		diff -= (1 << (sps.Log2MaxPicOrderCntLsbMinus4 + 4))
	}
	return diff
}

func getSEIPicTimingDPBOutputDelay(buf []byte, sps *SPS) (uint32, bool) {
	buf = AntiCompetitionRemove(buf)
	pos := 1

	for {
		if pos >= (len(buf) - 1) {
			return 0, false
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

			// cpbRemovalDelay
			_, err := bits.ReadBits(buf2, &pos2, int(sps.VUI.NalHRD.CpbRemovalDelayLengthMinus1+1))
			if err != nil {
				return 0, false
			}

			tmp, err := bits.ReadBits(buf2, &pos2, int(sps.VUI.NalHRD.DpbOutputDelayLengthMinus1+1))
			if err != nil {
				return 0, false
			}
			dpbOutputDelay := uint32(tmp)

			return dpbOutputDelay, true
		}

		pos += payloadSize
	}
}

func findSEIPicTimingDPBOutputDelay(nalus [][]byte, sps *SPS) (uint32, bool) {
	for _, nalu := range nalus {
		typ := NALUType(nalu[0] & 0x1F)
		if typ == NALUTypeSEI {
			ret, ok := getSEIPicTimingDPBOutputDelay(nalu, sps)
			if ok {
				return ret, true
			}
		}
	}
	return 0, false
}

// DTSExtractor is a utility that allows to extract NALU DTS from PTS.
type DTSExtractor struct {
	sps                 []byte
	spsp                *SPS
	prevPTS             time.Duration
	prevDTS             *time.Duration
	prevPOCDiff         int32
	expectedPOC         uint32
	ptsDTSOffset        time.Duration
	firstDPBOutputDelay *uint32
}

// NewDTSExtractor allocates a DTSExtractor.
func NewDTSExtractor() *DTSExtractor {
	return &DTSExtractor{}
}

func (d *DTSExtractor) extractInner(nalus [][]byte, pts time.Duration) (time.Duration, int32, error) {
	idrPresent := false

	for _, nalu := range nalus {
		typ := NALUType(nalu[0] & 0x1F)
		switch typ {
		case NALUTypeSPS:
			if d.sps == nil || !bytes.Equal(d.sps, nalu) {
				var spsp SPS
				err := spsp.Unmarshal(nalu)
				if err != nil {
					return 0, 0, fmt.Errorf("invalid SPS: %v", err)
				}
				d.sps = append([]byte(nil), nalu...)
				d.spsp = &spsp

				if d.spsp.VUI != nil && d.spsp.VUI.TimingInfo != nil &&
					d.spsp.VUI.BitstreamRestriction != nil {
					d.ptsDTSOffset = time.Duration(math.Round(float64(
						time.Duration(d.spsp.VUI.BitstreamRestriction.MaxNumReorderFrames)*time.Second*
							time.Duration(d.spsp.VUI.TimingInfo.NumUnitsInTick)*2) / float64(d.spsp.VUI.TimingInfo.TimeScale)))
				} else {
					d.ptsDTSOffset = 0
				}
			}

		case NALUTypeIDR:
			idrPresent = true
		}
	}

	if d.spsp == nil {
		return 0, 0, fmt.Errorf("SPS not received yet")
	}

	switch {
	// DTS is computed by using POC, timing infos and max_num_reorder_frames
	case d.spsp.PicOrderCntType != 2 &&
		d.spsp.VUI != nil && d.spsp.VUI.TimingInfo != nil && d.spsp.VUI.BitstreamRestriction != nil:
		if idrPresent {
			d.expectedPOC = 0
			return pts - d.ptsDTSOffset, 0, nil
		}

		// compute expectedPOC immediately in order to store it even in case of errors
		d.expectedPOC += 2
		d.expectedPOC &= ((1 << (d.spsp.Log2MaxPicOrderCntLsbMinus4 + 4)) - 1)

		poc, err := findPOC(nalus, d.spsp)
		if err != nil {
			return 0, 0, err
		}

		pocDiff := getPOCDiff(poc, d.expectedPOC, d.spsp)

		if pocDiff == 0 {
			return pts - d.ptsDTSOffset, 0, nil
		}

		// special case to eliminate errors near 0
		if pocDiff == -int32(d.spsp.VUI.BitstreamRestriction.MaxNumReorderFrames)*2 {
			return pts, pocDiff, nil
		}

		if d.prevPOCDiff == 0 {
			if pocDiff == -2 {
				return 0, 0, fmt.Errorf("invalid frame POC")
			}

			return d.prevPTS - d.ptsDTSOffset +
				time.Duration(math.Round(float64(pts-d.prevPTS)/float64(pocDiff/2+1))), pocDiff, nil
		}

		// pocDiff : prevPOCDiff = (pts - dts - ptsDTSOffset) : (prevPTS - prevDTS - ptsDTSOffset)
		return pts - d.ptsDTSOffset + time.Duration(math.Round(float64(*d.prevDTS-d.prevPTS+d.ptsDTSOffset)*
			float64(pocDiff)/float64(d.prevPOCDiff))), pocDiff, nil

	// DTS is computed from SEI
	case d.spsp.VUI != nil && d.spsp.VUI.TimingInfo != nil && d.spsp.VUI.NalHRD != nil:
		dpbOutputDelay, ok := findSEIPicTimingDPBOutputDelay(nalus, d.spsp)
		if !ok {
			// some streams declare that they use SEI pic timings, but they don't.
			// assume PTS = DTS.
			return pts, 0, nil
		}

		// workaround for nvenc
		// nvenc puts a wrong dpbOutputDelay in IDR frames that follows the first one.
		// save the dpbOutputDelay of the first IDR frame and use if for subsequent
		// IDR frames.
		// https://forums.developer.nvidia.com/t/nvcodec-h-264-encoder-sei-pic-timing-dpb-output-delay/156050
		// https://forums.developer.nvidia.com/t/h264-pic-timing-sei-message/71188
		if idrPresent &&
			d.spsp.VUI.NalHRD.CpbRemovalDelayLengthMinus1 == 15 &&
			d.spsp.VUI.NalHRD.DpbOutputDelayLengthMinus1 == 5 {
			if d.firstDPBOutputDelay == nil {
				d.firstDPBOutputDelay = &dpbOutputDelay
			} else {
				dpbOutputDelay = *d.firstDPBOutputDelay
			}
		}

		return pts - time.Duration(dpbOutputDelay)/2*time.Second*
			time.Duration(d.spsp.VUI.TimingInfo.NumUnitsInTick)*2/time.Duration(d.spsp.VUI.TimingInfo.TimeScale), 0, nil

	// we assume PTS = DTS
	default:
		return pts, 0, nil
	}
}

// Extract extracts the DTS of a group of NALUs.
func (d *DTSExtractor) Extract(nalus [][]byte, pts time.Duration) (time.Duration, error) {
	dts, pocDiff, err := d.extractInner(nalus, pts)
	if err != nil {
		return 0, err
	}

	if dts > pts {
		return 0, fmt.Errorf("DTS is greater than PTS")
	}

	if d.prevDTS != nil && dts <= *d.prevDTS {
		return 0, fmt.Errorf("DTS is not monotonically increasing (was %v, now is %v)",
			*d.prevDTS, dts)
	}

	d.prevPTS = pts
	d.prevDTS = &dts
	d.prevPOCDiff = pocDiff
	return dts, err
}
