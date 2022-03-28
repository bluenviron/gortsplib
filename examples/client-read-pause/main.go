package main

import (
	"log"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/pion/rtcp"
	"github.com/pion/rtp/v2"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. wait for 5 seconds
// 3. pause for 5 seconds
// 4. repeat

func main() {
	c := gortsplib.Client{
		// called when a RTP packet arrives
		OnPacketRTP: func(trackID int, pkt *rtp.Packet) {
			log.Printf("RTP packet from track %d, payload type %d\n", trackID, pkt.Header.PayloadType)
		},
		// called when a RTCP packet arrives
		OnPacketRTCP: func(trackID int, pkt rtcp.Packet) {
			log.Printf("RTCP packet from track %d, type %T\n", trackID, pkt)
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
