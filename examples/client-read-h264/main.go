package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtph264"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check whether there's an H264 track
// 3. get H264 NALUs of that track

func main() {
	// connect to the server and start reading all tracks
	conn, err := gortsplib.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// find the H264 track
	h264Track := func() int {
		for i, track := range conn.Tracks() {
			if track.IsH264() {
				return i
			}
		}
		return -1
	}()
	if h264Track < 0 {
		panic(fmt.Errorf("H264 track not found"))
	}
	fmt.Printf("H264 track is number %d\n", h264Track+1)

	// instantiate a RTP/H264 decoder
	dec := rtph264.NewDecoder()

	// read RTP packets
	err = conn.ReadFrames(func(trackID int, streamType gortsplib.StreamType, payload []byte) {
		if streamType == gortsplib.StreamTypeRTP && trackID == h264Track {
			// convert RTP packets into H264 NALUs
			nalus, _, err := dec.Decode(payload)
			if err != nil {
				return
			}

			// print NALUs
			for _, nalu := range nalus {
				fmt.Printf("received H264 NALU of size %d\n", len(nalu))
			}
		}
	})
	panic(err)
}
