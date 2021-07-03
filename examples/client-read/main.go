package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path

func main() {
	// connect to the server and start reading all tracks
	conn, err := gortsplib.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// read RTP frames
	err = conn.ReadFrames(func(trackID int, streamType gortsplib.StreamType, payload []byte) {
		fmt.Printf("frame from track %d, type %v, size %d\n", trackID, streamType, len(payload))
	})
	panic(err)
}
