package main

import (
	"bufio"
	"os"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/pkg/formats/mpegts"
)

func durationGoToMPEGTS(v time.Duration) int64 {
	return int64(v.Seconds() * 90000)
}

// mpegtsMuxer allows to save a H264 stream into a MPEG-TS file.
type mpegtsMuxer struct {
	fileName string
	sps      []byte
	pps      []byte

	f            *os.File
	b            *bufio.Writer
	w            *mpegts.Writer
	track        *mpegts.Track
	dtsExtractor *h264.DTSExtractor
}

// initialize initializes a mpegtsMuxer.
func (e *mpegtsMuxer) initialize() error {
	var err error
	e.f, err = os.Create(e.fileName)
	if err != nil {
		return err
	}
	e.b = bufio.NewWriter(e.f)

	e.track = &mpegts.Track{
		Codec: &mpegts.CodecH264{},
	}

	e.w = mpegts.NewWriter(e.b, []*mpegts.Track{e.track})

	return nil
}

// close closes all the mpegtsMuxer resources.
func (e *mpegtsMuxer) close() {
	e.b.Flush()
	e.f.Close()
}

// writeH264 writes a H264 access unit into MPEG-TS.
func (e *mpegtsMuxer) writeH264(au [][]byte, pts time.Duration) error {
	// prepend an AUD. This is required by some players
	filteredAU := [][]byte{
		{byte(h264.NALUTypeAccessUnitDelimiter), 240},
	}

	nonIDRPresent := false
	idrPresent := false

	for _, nalu := range au {
		typ := h264.NALUType(nalu[0] & 0x1F)
		switch typ {
		case h264.NALUTypeSPS:
			e.sps = nalu
			continue

		case h264.NALUTypePPS:
			e.pps = nalu
			continue

		case h264.NALUTypeAccessUnitDelimiter:
			continue

		case h264.NALUTypeIDR:
			idrPresent = true

		case h264.NALUTypeNonIDR:
			nonIDRPresent = true
		}

		filteredAU = append(filteredAU, nalu)
	}

	au = filteredAU

	if len(au) <= 1 || (!nonIDRPresent && !idrPresent) {
		return nil
	}

	// add SPS and PPS before access unit that contains an IDR
	if idrPresent {
		au = append([][]byte{e.sps, e.pps}, au...)
	}

	var dts time.Duration

	if e.dtsExtractor == nil {
		// skip samples silently until we find one with a IDR
		if !idrPresent {
			return nil
		}
		e.dtsExtractor = h264.NewDTSExtractor()
	}

	var err error
	dts, err = e.dtsExtractor.Extract(au, pts)
	if err != nil {
		return err
	}

	// encode into MPEG-TS
	return e.w.WriteH26x(e.track, durationGoToMPEGTS(pts), durationGoToMPEGTS(dts), idrPresent, au)
}
