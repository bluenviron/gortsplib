package main

import (
	"bufio"
	"os"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/h265"
	"github.com/bluenviron/mediacommon/pkg/formats/mpegts"
)

func durationGoToMPEGTS(v time.Duration) int64 {
	return int64(v.Seconds() * 90000)
}

// mpegtsMuxer allows to save a H265 stream into a MPEG-TS file.
type mpegtsMuxer struct {
	vps []byte
	sps []byte
	pps []byte

	f            *os.File
	b            *bufio.Writer
	w            *mpegts.Writer
	track        *mpegts.Track
	dtsExtractor *h265.DTSExtractor
}

// newMPEGTSMuxer allocates a mpegtsMuxer.
func newMPEGTSMuxer(vps []byte, sps []byte, pps []byte) (*mpegtsMuxer, error) {
	f, err := os.Create("mystream.ts")
	if err != nil {
		return nil, err
	}
	b := bufio.NewWriter(f)

	track := &mpegts.Track{
		Codec: &mpegts.CodecH265{},
	}

	w := mpegts.NewWriter(b, []*mpegts.Track{track})

	return &mpegtsMuxer{
		vps:   vps,
		sps:   sps,
		pps:   pps,
		f:     f,
		b:     b,
		w:     w,
		track: track,
	}, nil
}

// close closes all the mpegtsMuxer resources.
func (e *mpegtsMuxer) close() {
	e.b.Flush()
	e.f.Close()
}

// encode encodes a H265 access unit into MPEG-TS.
func (e *mpegtsMuxer) encode(au [][]byte, pts time.Duration) error {
	// prepend an AUD. This is required by some players
	filteredAU := [][]byte{
		{byte(h265.NALUType_AUD_NUT) << 1, 1, 0x50},
	}

	isRandomAccess := false

	for _, nalu := range au {
		typ := h265.NALUType((nalu[0] >> 1) & 0b111111)
		switch typ {
		case h265.NALUType_VPS_NUT:
			e.vps = nalu
			continue

		case h265.NALUType_SPS_NUT:
			e.sps = nalu
			continue

		case h265.NALUType_PPS_NUT:
			e.pps = nalu
			continue

		case h265.NALUType_AUD_NUT:
			continue

		case h265.NALUType_IDR_W_RADL, h265.NALUType_IDR_N_LP, h265.NALUType_CRA_NUT:
			isRandomAccess = true
		}

		filteredAU = append(filteredAU, nalu)
	}

	au = filteredAU

	if len(au) <= 1 {
		return nil
	}

	// add VPS, SPS and PPS before random access access unit
	if isRandomAccess {
		au = append([][]byte{e.vps, e.sps, e.pps}, au...)
	}

	var dts time.Duration

	if e.dtsExtractor == nil {
		// skip samples silently until we find one with a IDR
		if !isRandomAccess {
			return nil
		}
		e.dtsExtractor = h265.NewDTSExtractor()
	}

	var err error
	dts, err = e.dtsExtractor.Extract(au, pts)
	if err != nil {
		return err
	}

	// encode into MPEG-TS
	return e.w.WriteH26x(e.track, durationGoToMPEGTS(pts), durationGoToMPEGTS(dts), isRandomAccess, au)
}
