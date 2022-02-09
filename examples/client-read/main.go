package main

import (
	"log"

	"github.com/aler9/gortsplib"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path

func main() {
	c := gortsplib.Client{
		// called when a RTP packet arrives
		OnPacketRTP: func(trackID int, payload []byte) {
			log.Printf("RTP packet from track %d, size %d\n", trackID, len(payload))
		},
		// called when a RTCP packet arrives
		OnPacketRTCP: func(trackID int, payload []byte) {
			log.Printf("RTCP packet from track %d, size %d\n", trackID, len(payload))
		},
	}

	// connect to the server and start reading all tracks
	panic(c.StartReadingAndWait("rtsp://localhost:8554/mystream"))
}
