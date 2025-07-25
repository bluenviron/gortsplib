// Package main contains an example.
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
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. read H264 frames from a video file in MPEG-TS format.
// 2. connect to a RTSP server, announce a H264 format.
// 3. wrap frames into RTP packets.
// 4. write RTP packets to the server.

func findTrack(r *mpegts.Reader) (*mpegts.Track, error) {
	for _, track := range r.Tracks() {
		if _, ok := track.Codec.(*mpegts.CodecH264); ok {
			return track, nil
		}
	}
	return nil, fmt.Errorf("H264 track not found")
}

func randUint32() (uint32, error) {
	var b [4]byte
	_, err := rand.Read(b[:])
	if err != nil {
		return 0, err
	}
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), nil
}

func main() {
	// create a RTSP description that contains a H264 format
	forma := &format.H264{
		PayloadTyp:        96,
		PacketizationMode: 1,
	}
	desc := &description.Session{
		Medias: []*description.Media{{
			Type:    description.MediaTypeVideo,
			Formats: []format.Format{forma},
		}},
	}

	// connect to the server, announce the format and start recording
	c := gortsplib.Client{}
	err := c.StartRecording("rtsp://myuser:mypass@localhost:8554/mystream", desc)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// open a file in MPEG-TS format
	f, err := os.Open("myvideo.ts")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// setup H264 -> RTP encoder
	rtpEnc, err := forma.CreateEncoder()
	if err != nil {
		panic(err)
	}

	randomStart, err := randUint32()
	if err != nil {
		panic(err)
	}

	for {
		// setup MPEG-TS parser
		r := &mpegts.Reader{R: f}
		err = r.Initialize()
		if err != nil {
			panic(err)
		}

		// find the H264 track inside the file
		var track *mpegts.Track
		track, err = findTrack(r)
		if err != nil {
			panic(err)
		}

		timeDecoder := mpegts.TimeDecoder{}
		timeDecoder.Initialize()

		var firstDTS *int64
		var startTime time.Time
		var lastRTPTime uint32

		// setup a callback that is called when a H264 access unit is read from the file
		r.OnDataH264(track, func(pts, dts int64, au [][]byte) error {
			dts = timeDecoder.Decode(dts)
			pts = timeDecoder.Decode(pts)

			// sleep between access units
			if firstDTS != nil {
				timeDrift := time.Duration(dts-*firstDTS)*time.Second/90000 - time.Since(startTime)
				if timeDrift > 0 {
					time.Sleep(timeDrift)
				}
			} else {
				startTime = time.Now()
				firstDTS = &dts
			}

			log.Printf("writing access unit with pts=%d dts=%d", pts, dts)

			// wrap the access unit into RTP packets
			var packets []*rtp.Packet
			packets, err = rtpEnc.Encode(au)
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
				err = c.WritePacketRTP(desc.Medias[0], packet)
				if err != nil {
					return err
				}
			}

			return nil
		})

		// read the file
		for {
			err = r.Read()
			if err != nil {
				// file has ended
				if errors.Is(err, io.EOF) {
					log.Printf("file has ended, rewinding")

					// rewind to start position
					_, err = f.Seek(0, io.SeekStart)
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
