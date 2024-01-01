package main

import (
	"bufio"
	"os"
	"time"

	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
	"github.com/bluenviron/mediacommon/pkg/formats/mpegts"
)

func durationGoToMPEGTS(v time.Duration) int64 {
	return int64(v.Seconds() * 90000)
}

// mpegtsMuxer allows to save a MPEG4-audio stream into a MPEG-TS file.
type mpegtsMuxer struct {
	fileName string
	config   *mpeg4audio.Config

	f     *os.File
	b     *bufio.Writer
	w     *mpegts.Writer
	track *mpegts.Track
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
		Codec: &mpegts.CodecMPEG4Audio{
			Config: *e.config,
		},
	}

	e.w = mpegts.NewWriter(e.b, []*mpegts.Track{e.track})

	return nil
}

// close closes all the mpegtsMuxer resources.
func (e *mpegtsMuxer) close() {
	e.b.Flush()
	e.f.Close()
}

// writeMPEG4Audio writes MPEG-4 audio access units into MPEG-TS.
func (e *mpegtsMuxer) writeMPEG4Audio(aus [][]byte, pts time.Duration) error {
	return e.w.WriteMPEG4Audio(e.track, durationGoToMPEGTS(pts), aus)
}
