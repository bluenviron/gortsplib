package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path

func main() {
	c := gortsplib.Client{
		// called when a RTP packet arrives
		OnPacketRTP: func(c *gortsplib.Client, trackID int, payload []byte) {
			fmt.Printf("RTP packet from track %d, size %d\n", trackID, len(payload))
		},
		// called when a RTCP packet arrives
		OnPacketRTCP: func(c *gortsplib.Client, trackID int, payload []byte) {
			fmt.Printf("RTCP packet from track %d, size %d\n", trackID, len(payload))
		},
	}

	// connect to the server and start reading all tracks
	err := c.DialRead("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// read packets
	err = c.ReadFrames()
	panic(err)
}
