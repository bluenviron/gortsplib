// +build ignore

package main

import (
	"fmt"
	"time"

	"github.com/aler9/gortsplib"
)

// This example shows how to
// * connect to a RTSP server
// * read all tracks on a path

func main() {
	// Dialer allows to set additional options
	dialer := gortsplib.Dialer{
		// the stream protocol
		StreamProtocol: gortsplib.StreamProtocolUDP,
		// timeout of read operations
		ReadTimeout: 10 * time.Second,
		// timeout of write operations
		WriteTimeout: 10 * time.Second,
		// read buffer count.
		// If greater than 1, allows to pass buffers to routines different than the one
		// that is reading frames
		ReadBufferCount: 1,
	}

	// connect to the server and start reading all tracks
	conn, err := dialer.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// read frames from the server
	readerDone := conn.OnFrame(func(id int, typ gortsplib.StreamType, buf []byte) {
		fmt.Printf("frame from track %d, type %v: %v\n", id, typ, buf)
	})

	<-readerDone
}
