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

	readerDone := make(chan struct{})
	defer func() { <-readerDone }()

	// read frames
	conn.OnFrame(func(id int, typ gortsplib.StreamType, buf []byte, err error) {
		if err != nil {
			fmt.Printf("ERR: %v\n", err)
			close(readerDone)
			return
		}

		fmt.Printf("frame from track %d, type %v: %v\n",
			id, typ, buf)
	})
}
