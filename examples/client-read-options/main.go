package main

import (
	"log"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. set additional client options
// 2. connect to a RTSP server and read all medias on a path

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
		OnPacketRTP: func(ctx *gortsplib.ClientOnPacketRTPCtx) {
			log.Printf("RTP packet from media %d, payload type %d\n", ctx.MediaID, ctx.Packet.Header.PayloadType)
		},
		// called when a RTCP packet arrives
		OnPacketRTCP: func(ctx *gortsplib.ClientOnPacketRTCPCtx) {
			log.Printf("RTCP packet from media %d, type %T\n", ctx.MediaID, ctx.Packet)
		},
	}

	// parse URL
	u, err := url.Parse("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	// find published medias
	medias, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// setup and read all medias
	err = c.SetupAndPlay(medias, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
