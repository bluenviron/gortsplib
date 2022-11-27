package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server and read all medias on a path
// 2. re-publish all tracks on another path.

func main() {
	reader := gortsplib.Client{}

	// parse source URL
	sourceURL, err := url.Parse("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = reader.Start(sourceURL.Scheme, sourceURL.Host)
	if err != nil {
		panic(err)
	}
	defer reader.Close()

	// find published medias
	medias, baseURL, _, err := reader.Describe(sourceURL)
	if err != nil {
		panic(err)
	}

	log.Printf("republishing %d medias", len(medias))

	publisher := gortsplib.Client{}

	// connect to the server and start publishing the same medias
	err = publisher.StartPublishing("rtsp://localhost:8554/mystream2", medias)
	if err != nil {
		panic(err)
	}
	defer publisher.Close()

	// called when a RTP packet arrives
	reader.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		publisher.WritePacketRTP(ctx.MediaID, ctx.Packet)
	}

	// setup and read all medias
	err = reader.SetupAndPlay(medias, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(reader.Wait())
}
