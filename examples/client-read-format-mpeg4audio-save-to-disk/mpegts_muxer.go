package main

import (
	"bufio"
	"log"
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
	config *mpeg4audio.Config
	f      *os.File
	b      *bufio.Writer
	w      *mpegts.Writer
	track  *mpegts.Track
}

// newMPEGTSMuxer allocates a mpegtsMuxer.
func newMPEGTSMuxer(config *mpeg4audio.Config) (*mpegtsMuxer, error) {
	f, err := os.Create("mystream.ts")
	if err != nil {
		return nil, err
	}
	b := bufio.NewWriter(f)

	track := &mpegts.Track{
		PID: 256,
		Codec: &mpegts.CodecMPEG4Audio{
			Config: *config,
		},
	}

	w := mpegts.NewWriter(b, []*mpegts.Track{track})

	return &mpegtsMuxer{
		config: config,
		f:      f,
		b:      b,
		w:      w,
		track:  track,
	}, nil
}

// close closes all the mpegtsMuxer resources.
func (e *mpegtsMuxer) close() {
	e.b.Flush()
	e.f.Close()
}

// encode encodes a MPEG4-audio access unit into MPEG-TS.
func (e *mpegtsMuxer) encode(au []byte, pts time.Duration) error {
	// encode into MPEG-TS
	err := e.w.WriteMPEG4Audio(e.track, durationGoToMPEGTS(pts), [][]byte{au})
	if err != nil {
		return err
	}

	log.Println("wrote TS packet")
	return nil
}
