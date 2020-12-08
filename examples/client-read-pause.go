// +build ignore

package main

import (
	"fmt"
	"time"

	"github.com/aler9/gortsplib"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. read all tracks for 5 seconds
// 3. pause for 5 seconds
// 4. repeat

func main() {
	// connect to the server and start reading all tracks
	conn, err := gortsplib.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	for {
		// read track frames
		done := conn.ReadFrames(func(id int, typ gortsplib.StreamType, buf []byte) {
			fmt.Printf("frame from track %d, type %v: %v\n", id, typ, buf)
		})

		// wait
		time.Sleep(5 * time.Second)

		// pause
		_, err := conn.Pause()
		if err != nil {
			panic(err)
		}

		// join reader
		<-done

		// wait
		time.Sleep(5 * time.Second)

		// play again
		_, err = conn.Play()
		if err != nil {
			panic(err)
		}
	}
}
