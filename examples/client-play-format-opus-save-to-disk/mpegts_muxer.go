package main

import (
	"bufio"
	"os"

	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
)

func multiplyAndDivide(v, m, d int64) int64 {
	secs := v / d
	dec := v % d
	return (secs*m + dec*m/d)
}

// mpegtsMuxer allows to save a MPEG-4 audio stream into a MPEG-TS file.
type mpegtsMuxer struct {
	fileName string
	format   format.Format
	track    *mpegts.Track

	f *os.File
	b *bufio.Writer
	w *mpegts.Writer
}

// initialize initializes a mpegtsMuxer.
func (e *mpegtsMuxer) initialize() error {
	var err error
	e.f, err = os.Create(e.fileName)
	if err != nil {
		return err
	}
	e.b = bufio.NewWriter(e.f)

	e.w = mpegts.NewWriter(e.b, []*mpegts.Track{e.track})

	return nil
}

// close closes all the mpegtsMuxer resources.
func (e *mpegtsMuxer) close() {
	e.b.Flush()
	e.f.Close()
}

// writeOpus writes Opus packets into MPEG-TS.
func (e *mpegtsMuxer) writeOpus(pkt []byte, pts int64) error {
	return e.w.WriteOpus(e.track, multiplyAndDivide(pts, 90000, int64(e.format.ClockRate())), [][]byte{pkt})
}
