package h264

import (
	"bytes"
	"fmt"
	"math"
	"time"

	"github.com/icza/bitio"
)

func getPOC(buf []byte, sps *SPS) (uint32, error) {
	buf = AntiCompetitionRemove(buf[:5])

	isIDR := NALUType(buf[0]&0x1F) == NALUTypeIDR

	r := bytes.NewReader(buf[1:])
	br := bitio.NewReader(r)

	// first_mb_in_slice
	_, err := readGolombUnsigned(br)
	if err != nil {
		return 0, err
	}

	// slice_type
	_, err = readGolombUnsigned(br)
	if err != nil {
		return 0, err
	}

	// pic_parameter_set_id
	_, err = readGolombUnsigned(br)
	if err != nil {
		return 0, err
	}

	// frame_num
	_, err = br.ReadBits(uint8(sps.Log2MaxFrameNumMinus4 + 4))
	if err != nil {
		return 0, err
	}

	if !sps.FrameMbsOnlyFlag {
		return 0, fmt.Errorf("unsupported")
	}

	if isIDR {
		// idr_pic_id
		_, err := readGolombUnsigned(br)
		if err != nil {
			return 0, err
		}
	}

	var picOrderCntLsb uint64
	switch {
	case sps.PicOrderCntType == 0:
		picOrderCntLsb, err = br.ReadBits(uint8(sps.Log2MaxPicOrderCntLsbMinus4 + 4))
		if err != nil {
			return 0, err
		}

	default:
		return 0, fmt.Errorf("pic_order_cnt_type = 1 is unsupported")
	}

	return uint32(picOrderCntLsb), nil
}

func getNALUSPOC(nalus [][]byte, sps *SPS) (uint32, error) {
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

// DTSExtractor is a utility that allows to extract NALU DTS from PTS.
type DTSExtractor struct {
	sps         []byte
	spsp        *SPS
	prevPTS     time.Duration
	prevDTS     time.Duration
	prevPOCDiff int32
	expectedPOC uint32
}

// NewDTSExtractor allocates a DTSExtractor.
func NewDTSExtractor() *DTSExtractor {
	return &DTSExtractor{}
}

func (d *DTSExtractor) extractInner(
	nalus [][]byte,
	pts time.Duration,
) (time.Duration, int32, error) {
	idrPresent := false

	for _, nalu := range nalus {
		typ := NALUType(nalu[0] & 0x1F)
		switch typ {
		// parse SPS
		case NALUTypeSPS:
			if d.sps == nil || !bytes.Equal(d.sps, nalu) {
				var spsp SPS
				err := spsp.Unmarshal(nalu)
				if err != nil {
					return 0, 0, err
				}
				d.sps = append([]byte(nil), nalu...)
				d.spsp = &spsp
			}

		// set IDR present flag
		case NALUTypeIDR:
			idrPresent = true
		}
	}

	if d.spsp == nil {
		return 0, 0, fmt.Errorf("SPS not received yet")
	}

	if idrPresent || d.spsp.PicOrderCntType == 2 {
		d.expectedPOC = 0
		return pts, 0, nil
	}

	// compute expectedPOC immediately in order to store it even in case of errors
	d.expectedPOC += 2
	d.expectedPOC &= ((1 << (d.spsp.Log2MaxPicOrderCntLsbMinus4 + 4)) - 1)

	poc, err := getNALUSPOC(nalus, d.spsp)
	if err != nil {
		return 0, 0, err
	}

	pocDiff := getPOCDiff(poc, d.expectedPOC, d.spsp)

	if pocDiff == 0 {
		return pts, pocDiff, nil
	}

	if d.prevPOCDiff == 0 {
		if pocDiff == -2 {
			return 0, 0, fmt.Errorf("invalid frame POC")
		}

		return d.prevPTS + time.Duration(math.Round(float64(pts-d.prevPTS)/float64(pocDiff/2+1))), pocDiff, nil
	}

	// pocDiff : prevPOCDiff = (pts - dts) : (prevPTS - prevDTS)
	return pts + time.Duration(math.Round(float64(d.prevDTS-d.prevPTS)*float64(pocDiff)/float64(d.prevPOCDiff))),
		pocDiff, nil
}

// Extract extracts the DTS of a NALU group.
func (d *DTSExtractor) Extract(
	nalus [][]byte,
	pts time.Duration,
) (time.Duration, error) {
	dts, pocDiff, err := d.extractInner(nalus, pts)
	if err != nil {
		return 0, err
	}

	d.prevPTS = pts
	d.prevDTS = dts
	d.prevPOCDiff = pocDiff
	return dts, err
}
