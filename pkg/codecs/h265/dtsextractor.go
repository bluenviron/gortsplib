package h265

import (
	"fmt"
	"time"

	"github.com/aler9/gortsplib/v2/pkg/bits"
	"github.com/aler9/gortsplib/v2/pkg/codecs/h264"
)

func getPictureOrderCount(buf []byte, sps *SPS, pps *PPS) (uint32, uint32, error) {
	buf = h264.EmulationPreventionRemove(buf[:12])

	typ := NALUType((buf[0] >> 1) & 0b111111)

	buf = buf[2:]
	pos := 0

	firstSliceSegmentInPicFlag, err := bits.ReadFlag(buf, &pos)
	if err != nil {
		return 0, 0, err
	}

	if !firstSliceSegmentInPicFlag {
		return 0, 0, fmt.Errorf("first_slice_segment_in_pic_flag = 0 is not supported")
	}

	if typ >= NALUType_BLA_W_LP && typ <= NALUType_RSV_IRAP_VCL23 {
		_, err := bits.ReadFlag(buf, &pos) // no_output_of_prior_pics_flag
		if err != nil {
			return 0, 0, err
		}
	}

	_, err = bits.ReadGolombUnsigned(buf, &pos) // slice_pic_parameter_set_id
	if err != nil {
		return 0, 0, err
	}

	if pps.NumExtraSliceHeaderBits > 0 {
		err := bits.HasSpace(buf, pos, int(pps.NumExtraSliceHeaderBits))
		if err != nil {
			return 0, 0, err
		}
		pos += int(pps.NumExtraSliceHeaderBits)
	}

	sliceType, err := bits.ReadGolombUnsigned(buf, &pos) // slice_type
	if err != nil {
		return 0, 0, err
	}

	if pps.OutputFlagPresentFlag {
		_, err := bits.ReadFlag(buf, &pos) // pic_output_flag
		if err != nil {
			return 0, 0, err
		}
	}

	if sps.SeparateColourPlaneFlag {
		_, err := bits.ReadBits(buf, &pos, 2) // colour_plane_id
		if err != nil {
			return 0, 0, err
		}
	}

	picOrderCntLsb, err := bits.ReadBits(buf, &pos, int(sps.Log2MaxPicOrderCntLsbMinus4+4))
	if err != nil {
		return 0, 0, err
	}

	shortTermRefPicSetSpsFlag, err := bits.ReadFlag(buf, &pos)
	if err != nil {
		return 0, 0, err
	}

	if shortTermRefPicSetSpsFlag {
		return 0, 0, fmt.Errorf("short_term_ref_pic_set_sps_flag = true is not supported")
	}

	var rps SPS_ShortTermRefPicSet
	err = rps.unmarshal(buf, &pos, uint32(len(sps.ShortTermRefPicSets)), uint32(len(sps.ShortTermRefPicSets)), nil)
	if err != nil {
		return 0, 0, err
	}

	var v uint32

	if sliceType == 0 { // B-frame
		if typ == NALUType_TRAIL_N || typ == NALUType_RASL_N {
			v = sps.MaxNumReorderPics[0] - uint32(len(rps.DeltaPocS1Minus1))
		} else if typ == NALUType_TRAIL_R || typ == NALUType_RASL_R {
			v = rps.DeltaPocS0Minus1[0] + sps.MaxNumReorderPics[0] - 1
		}
	} else { // I or P-frame
		v = rps.DeltaPocS0Minus1[0] + sps.MaxNumReorderPics[0]
	}

	dtsPOC := uint32(picOrderCntLsb) - v
	dtsPOC &= ((1 << (sps.Log2MaxPicOrderCntLsbMinus4 + 4)) - 1)

	return uint32(picOrderCntLsb), dtsPOC, nil
}

func findPictureOrderCount(au [][]byte, sps *SPS, pps *PPS) (uint32, uint32, error) {
	for _, nalu := range au {
		typ := NALUType((nalu[0] >> 1) & 0b111111)
		switch typ {
		case NALUType_TRAIL_N, NALUType_TRAIL_R, NALUType_CRA_NUT, NALUType_RASL_N, NALUType_RASL_R:
			poc, dtsPOC, err := getPictureOrderCount(nalu, sps, pps)
			if err != nil {
				return 0, 0, err
			}
			return poc, dtsPOC, nil
		}
	}
	return 0, 0, fmt.Errorf("POC not found")
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
	spsp          *SPS
	ppsp          *PPS
	prevDTSFilled bool
	prevDTS       time.Duration
}

// NewDTSExtractor allocates a DTSExtractor.
func NewDTSExtractor() *DTSExtractor {
	return &DTSExtractor{}
}

func (d *DTSExtractor) extractInner(au [][]byte, pts time.Duration) (time.Duration, error) {
	idrPresent := false

	for _, nalu := range au {
		typ := NALUType((nalu[0] >> 1) & 0b111111)

		switch typ {
		case NALUType_SPS_NUT:
			var spsp SPS
			err := spsp.Unmarshal(nalu)
			if err != nil {
				return 0, fmt.Errorf("invalid SPS: %v", err)
			}
			d.spsp = &spsp

		case NALUType_PPS_NUT:
			var ppsp PPS
			err := ppsp.Unmarshal(nalu)
			if err != nil {
				return 0, fmt.Errorf("invalid PPS: %v", err)
			}
			d.ppsp = &ppsp

		case NALUType_IDR_W_RADL, NALUType_IDR_N_LP:
			idrPresent = true
		}
	}

	if d.spsp == nil {
		return 0, fmt.Errorf("SPS not received yet")
	}

	if d.ppsp == nil {
		return 0, fmt.Errorf("PPS not received yet")
	}

	if len(d.spsp.MaxNumReorderPics) != 1 || d.spsp.MaxNumReorderPics[0] == 0 {
		return pts, nil
	}

	var poc uint32
	var dtsPOC uint32

	if idrPresent {
		poc = 0
		dtsPOC = poc - 2
		dtsPOC &= ((1 << (d.spsp.Log2MaxPicOrderCntLsbMinus4 + 4)) - 1)
	} else {
		var err error
		poc, dtsPOC, err = findPictureOrderCount(au, d.spsp, d.ppsp)
		if err != nil {
			return 0, err
		}
	}

	pocDiff := getPictureOrderCountDiff(poc, dtsPOC, d.spsp)
	timeDiff := time.Duration(pocDiff) * time.Second *
		time.Duration(d.spsp.VUI.TimingInfo.NumUnitsInTick) / time.Duration(d.spsp.VUI.TimingInfo.TimeScale)
	dts := pts - timeDiff

	return dts, nil
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

	d.prevDTSFilled = true
	d.prevDTS = dts

	return dts, err
}
