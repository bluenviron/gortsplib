package main

import (
	"log"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/pion/rtcp"
	"github.com/pion/rtp/v2"
)

// This example shows how to
// 1. set additional client options
// 2. connect to a RTSP server and read all tracks on a path

func main() {
	// Client allows to set additional client options
	c := &gortsplib.Client{
		// the stream transport (UDP, Multicast or TCP). If nil, it is chosen automatically
		Transport: nil,
		// timeout of read operations
		ReadTimeout: 10 * time.Second,
		// timeout of write operations
		WriteTimeout: 10 * time.Second,
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
	panic(c.StartReadingAndWait("rtsp://localhost:8554/mystream"))
}
