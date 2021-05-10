package main

import (
	"fmt"
	"time"

	"github.com/aler9/gortsplib"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. wait for 5 seconds
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
		// read RTP frames
		done := make(chan struct{})
		go func() {
			defer close(done)
			conn.ReadFrames(func(trackID int, typ gortsplib.StreamType, buf []byte) {
				fmt.Printf("frame from track %d, type %v, size %d\n", trackID, typ, len(buf))
			})
		}()

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
