package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtph264"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check whether there's a H264 track
// 3. get H264 NALUs of that track

func main() {
	// connect to the server and start reading all tracks
	conn, err := gortsplib.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// check whether there's a H264 track
	h264Track := func() int {
		for _, track := range conn.Tracks() {
			if track.IsH264() {
				return track.ID
			}
		}
		return -1
	}()
	if h264Track < 0 {
		panic(fmt.Errorf("H264 track not found"))
	}
	fmt.Printf("H264 track is number %d\n", h264Track+1)

	// instantiate a decoder
	dec := rtph264.NewDecoder()

	// read RTP frames
	err = <-conn.ReadFrames(func(trackID int, typ gortsplib.StreamType, buf []byte) {
		if trackID == h264Track {
			// convert RTP frames into H264 NALUs
			nts, err := dec.Decode(buf)
			if err != nil {
				return
			}

			// print NALUs
			for _, nt := range nts {
				fmt.Printf("received H264 NALU of size %d\n", len(nt.NALU))
			}
		}
	})
	panic(err)
}
