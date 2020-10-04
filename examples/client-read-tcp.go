// +build ignore

package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
)

// This example shows how to create a RTSP client, connect to a server and
// read all tracks with the TCP protocol.

func main() {
	// connect to the server and start reading all tracks
	conn, _, err := gortsplib.DialRead("rtsp://localhost:8554/mystream", gortsplib.StreamProtocolTCP)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	for {
		// read frames
		frame, err := conn.ReadFrameTCP()
		if err != nil {
			fmt.Println("connection is closed (%s)", err)
			break
		}

		fmt.Printf("frame from track %d, type %v: %v\n",
			frame.TrackId, frame.StreamType, frame.Content)
	}
}
