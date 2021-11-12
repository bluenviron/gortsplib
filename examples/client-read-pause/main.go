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
	c := gortsplib.Client{
		// called when a RTP packet arrives
		OnPacketRTP: func(trackID int, payload []byte) {
			fmt.Printf("RTP packet from track %d, size %d\n", trackID, len(payload))
		},
		// called when a RTCP packet arrives
		OnPacketRTCP: func(trackID int, payload []byte) {
			fmt.Printf("RTCP packet from track %d, size %d\n", trackID, len(payload))
		},
	}

	// connect to the server and start reading all tracks
	err := c.StartReading("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	for {
		// wait
		time.Sleep(5 * time.Second)

		// pause
		_, err := c.Pause()
		if err != nil {
			panic(err)
		}

		// wait
		time.Sleep(5 * time.Second)

		// play again
		_, err = c.Play(nil)
		if err != nil {
			panic(err)
		}
	}
}
