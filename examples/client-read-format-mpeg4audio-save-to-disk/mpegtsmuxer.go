package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"time"

	"github.com/asticode/go-astits"
	"github.com/bluenviron/mediacommon/pkg/codecs/mpeg4audio"
)

// mpegtsMuxer allows to save a MPEG4-audio stream into a MPEG-TS file.
type mpegtsMuxer struct {
	config *mpeg4audio.Config
	f      *os.File
	b      *bufio.Writer
	mux    *astits.Muxer
}

// newMPEGTSMuxer allocates a mpegtsMuxer.
func newMPEGTSMuxer(config *mpeg4audio.Config) (*mpegtsMuxer, error) {
	f, err := os.Create("mystream.ts")
	if err != nil {
		return nil, err
	}
	b := bufio.NewWriter(f)

	mux := astits.NewMuxer(context.Background(), b)
	mux.AddElementaryStream(astits.PMTElementaryStream{
		ElementaryPID: 257,
		StreamType:    astits.StreamTypeAACAudio,
	})
	mux.SetPCRPID(257)

	return &mpegtsMuxer{
		config: config,
		f:      f,
		b:      b,
		mux:    mux,
	}, nil
}

// close closes all the mpegtsMuxer resources.
func (e *mpegtsMuxer) close() {
	e.b.Flush()
	e.f.Close()
}

// encode encodes a MPEG4-audio access unit into MPEG-TS.
func (e *mpegtsMuxer) encode(au []byte, pts time.Duration) error {
	// wrap access unit inside an ADTS packet
	pkts := mpeg4audio.ADTSPackets{
		{
			Type:         e.config.Type,
			SampleRate:   e.config.SampleRate,
			ChannelCount: e.config.ChannelCount,
			AU:           au,
		},
	}
	enc, err := pkts.Marshal()
	if err != nil {
		return err
	}

	_, err = e.mux.WriteData(&astits.MuxerData{
		PID: 257,
		AdaptationField: &astits.PacketAdaptationField{
			RandomAccessIndicator: true,
		},
		PES: &astits.PESData{
			Header: &astits.PESHeader{
				OptionalHeader: &astits.PESOptionalHeader{
					MarkerBits:      2,
					PTSDTSIndicator: astits.PTSDTSIndicatorOnlyPTS,
					PTS:             &astits.ClockReference{Base: int64(pts.Seconds() * 90000)},
				},
				PacketLength: uint16(len(enc) + 8),
				StreamID:     192, // audio
			},
			Data: enc,
		},
	})
	if err != nil {
		return err
	}

	log.Println("wrote TS packet")
	return nil
}
