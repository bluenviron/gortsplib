package main

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
)

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

func findTrack(r *mpegts.Reader) (*mpegts.Track, error) {
	for _, track := range r.Tracks() {
		if _, ok := track.Codec.(*mpegts.CodecH264); ok {
			return track, nil
		}
	}
	return nil, fmt.Errorf("H264 track not found")
}

type fileStreamer struct {
	stream *gortsplib.ServerStream

	f *os.File
}

func (r *fileStreamer) initialize() error {
	// open a file in MPEG-TS format
	var err error
	r.f, err = os.Open("myvideo.ts")
	if err != nil {
		return err
	}

	// in a separate routine, route frames from file to ServerStream
	go r.run()

	return nil
}

func (r *fileStreamer) close() {
	r.f.Close()
}

func (r *fileStreamer) run() {
	// setup H264 -> RTP encoder
	rtpEnc, err := r.stream.Desc.Medias[0].Formats[0].(*format.H264).CreateEncoder()
	if err != nil {
		panic(err)
	}

	randomStart, err := randUint32()
	if err != nil {
		panic(err)
	}

	for {
		// setup MPEG-TS parser
		mr := &mpegts.Reader{R: r.f}
		err = mr.Initialize()
		if err != nil {
			panic(err)
		}

		// find the H264 track inside the file
		track, err := findTrack(mr)
		if err != nil {
			panic(err)
		}

		timeDecoder := mpegts.TimeDecoder{}
		timeDecoder.Initialize()

		var firstDTS *int64
		var firstTime time.Time
		var lastRTPTime uint32

		// setup a callback that is called when a H264 access unit is read from the file
		mr.OnDataH264(track, func(pts, dts int64, au [][]byte) error {
			dts = timeDecoder.Decode(dts)
			pts = timeDecoder.Decode(pts)

			// sleep between access units
			if firstDTS != nil {
				timeDrift := time.Duration(dts-*firstDTS)*time.Second/90000 - time.Since(firstTime)
				if timeDrift > 0 {
					time.Sleep(timeDrift)
				}
			} else {
				firstTime = time.Now()
				firstDTS = &dts
			}

			log.Printf("writing access unit with pts=%d dts=%d", pts, dts)

			// wrap the access unit into RTP packets
			packets, err := rtpEnc.Encode(au)
			if err != nil {
				return err
			}

			// set packet timestamp
			// we don't have to perform any conversion
			// since H264 clock rate is the same in both MPEG-TS and RTSP
			lastRTPTime = uint32(int64(randomStart) + pts)
			for _, packet := range packets {
				packet.Timestamp = lastRTPTime
			}

			// write RTP packets to the server
			for _, packet := range packets {
				err := r.stream.WritePacketRTP(r.stream.Desc.Medias[0], packet)
				if err != nil {
					return err
				}
			}

			return nil
		})

		// read the file
		for {
			err := mr.Read()
			if err != nil {
				// file has ended
				if errors.Is(err, io.EOF) {
					log.Printf("file has ended, rewinding")

					// rewind to start position
					_, err = r.f.Seek(0, io.SeekStart)
					if err != nil {
						panic(err)
					}

					// keep current timestamp
					randomStart = lastRTPTime + 1

					break
				}
				panic(err)
			}
		}
	}
}
