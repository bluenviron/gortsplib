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
	dialer := gortsplib.Dialer{
		StreamProtocol: gortsplib.StreamProtocolTCP,
	}
	conn, err := dialer.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// read frames
	for {
		id, typ, buf, err := conn.ReadFrame()
		if err != nil {
			fmt.Printf("connection is closed (%s)\n", err)
			break
		}

		fmt.Printf("frame from track %d, type %v: %v\n",
			id, typ, buf)
	}
}
