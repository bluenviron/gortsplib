// +build ignore

package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
)

// This example shows how to
// * connect to a RTSP server
// * read all tracks with the UDP protocol

func main() {
	// connect to the server and start reading all tracks
	conn, err := gortsplib.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// read frames from the server
	done := conn.OnFrame(func(id int, typ gortsplib.StreamType, buf []byte) {
		fmt.Printf("frame from track %d, type %v: %v\n", id, typ, buf)
	})

	<-done
}
