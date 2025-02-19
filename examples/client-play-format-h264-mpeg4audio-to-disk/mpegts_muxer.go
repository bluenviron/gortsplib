package main

import (
	"bufio"
	"os"
	"sync"

	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/h264"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
)

func multiplyAndDivide(v, m, d int64) int64 {
	secs := v / d
	dec := v % d
	return (secs*m + dec*m/d)
}

// mpegtsMuxer allows to save a H264 / MPEG-4 audio stream into a MPEG-TS file.
type mpegtsMuxer struct {
	fileName         string
	h264Format       *format.H264
	mpeg4AudioFormat *format.MPEG4Audio

	f               *os.File
	b               *bufio.Writer
	w               *mpegts.Writer
	h264Track       *mpegts.Track
	mpeg4AudioTrack *mpegts.Track
	dtsExtractor    *h264.DTSExtractor
	mutex           sync.Mutex
}

// initialize initializes a mpegtsMuxer.
func (e *mpegtsMuxer) initialize() error {
	var err error
	e.f, err = os.Create(e.fileName)
	if err != nil {
		return err
	}
	e.b = bufio.NewWriter(e.f)

	e.h264Track = &mpegts.Track{
		Codec: &mpegts.CodecH264{},
	}

	e.mpeg4AudioTrack = &mpegts.Track{
		Codec: &mpegts.CodecMPEG4Audio{
			Config: *e.mpeg4AudioFormat.Config,
		},
	}

	e.w = mpegts.NewWriter(e.b, []*mpegts.Track{e.h264Track, e.mpeg4AudioTrack})

	return nil
}

// close closes all the mpegtsMuxer resources.
func (e *mpegtsMuxer) close() {
	e.b.Flush()
	e.f.Close()
}

// writeH264 writes a H264 access unit into MPEG-TS.
func (e *mpegtsMuxer) writeH264(au [][]byte, pts int64) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	var filteredAU [][]byte

	nonIDRPresent := false
	idrPresent := false

	for _, nalu := range au {
		typ := h264.NALUType(nalu[0] & 0x1F)
		switch typ {
		case h264.NALUTypeSPS:
			e.h264Format.SPS = nalu
			continue

		case h264.NALUTypePPS:
			e.h264Format.PPS = nalu
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

	if au == nil || (!nonIDRPresent && !idrPresent) {
		return nil
	}

	// add SPS and PPS before access unit that contains an IDR
	if idrPresent {
		au = append([][]byte{e.h264Format.SPS, e.h264Format.PPS}, au...)
	}

	if e.dtsExtractor == nil {
		// skip samples silently until we find one with a IDR
		if !idrPresent {
			return nil
		}
		e.dtsExtractor = h264.NewDTSExtractor()
	}

	dts, err := e.dtsExtractor.Extract(au, pts)
	if err != nil {
		return err
	}

	// encode into MPEG-TS
	return e.w.WriteH264(e.h264Track, pts, dts, au)
}

// writeMPEG4Audio writes MPEG-4 audio access units into MPEG-TS.
func (e *mpegtsMuxer) writeMPEG4Audio(aus [][]byte, pts int64) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	return e.w.WriteMPEG4Audio(e.mpeg4AudioTrack, multiplyAndDivide(pts, 90000, int64(e.mpeg4AudioFormat.ClockRate())), aus)
}
