package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path

func main() {
	c := gortsplib.Client{}

	// connect to the server and start reading all tracks
	conn, err := c.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// read packets
	err = conn.ReadFrames(func(trackID int, streamType gortsplib.StreamType, payload []byte) {
		fmt.Printf("packet from track %d, type %v, size %d\n", trackID, streamType, len(payload))
	})
	panic(err)
}
