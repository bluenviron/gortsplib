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
	c := gortsplib.Client{}

	// connect to the server and start reading all tracks
	conn, err := c.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	for {
		// read packets
		done := make(chan struct{})
		go func() {
			defer close(done)
			conn.ReadFrames(func(trackID int, streamType gortsplib.StreamType, payload []byte) {
				fmt.Printf("packet from track %d, type %v, size %d\n", trackID, streamType, len(payload))
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
		_, err = conn.Play(nil)
		if err != nil {
			panic(err)
		}
	}
}
