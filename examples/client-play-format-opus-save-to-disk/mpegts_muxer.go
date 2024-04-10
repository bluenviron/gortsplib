package main

import (
	"bufio"
	"os"
	"time"

	"github.com/bluenviron/mediacommon/pkg/formats/mpegts"
)

func durationGoToMPEGTS(v time.Duration) int64 {
	return int64(v.Seconds() * 90000)
}

// mpegtsMuxer allows to save a MPEG-4 audio stream into a MPEG-TS file.
type mpegtsMuxer struct {
	fileName string
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
func (e *mpegtsMuxer) writeOpus(pkt []byte, pts time.Duration) error {
	return e.w.WriteOpus(e.track, durationGoToMPEGTS(pts), [][]byte{pkt})
}
