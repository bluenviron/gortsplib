package main

import (
	"log"

	"github.com/aler9/gortsplib"
)

// This example shows how to connect to a RTSP server
// and read all tracks on a path.

func main() {
	c := gortsplib.Client{
		// called when a RTP packet arrives
		OnPacketRTP: func(ctx *gortsplib.ClientOnPacketRTPCtx) {
			log.Printf("RTP packet from track %d, payload type %d\n", ctx.TrackID, ctx.Packet.Header.PayloadType)
		},
		// called when a RTCP packet arrives
		OnPacketRTCP: func(ctx *gortsplib.ClientOnPacketRTCPCtx) {
			log.Printf("RTCP packet from track %d, type %T\n", ctx.TrackID, ctx.Packet)
		},
	}

	// connect to the server and start reading all tracks
	panic(c.StartReadingAndWait("rtsp://localhost:8554/mystream"))
}
