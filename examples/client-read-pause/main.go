package main

import (
	"log"
	"time"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server and read all medias on a path
// 2. wait for 5 seconds
// 3. pause for 5 seconds
// 4. repeat

func main() {
	c := gortsplib.Client{
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
