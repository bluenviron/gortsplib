package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. re-publish all tracks on another path.

func main() {
	reader := gortsplib.Client{}

	// parse source URL
	sourceURL, err := base.ParseURL("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = reader.Start(sourceURL.Scheme, sourceURL.Host)
	if err != nil {
		panic(err)
	}
	defer reader.Close()

	// find published tracks
	tracks, baseURL, _, err := reader.Describe(sourceURL)
	if err != nil {
		panic(err)
	}

	log.Printf("republishing %d tracks", len(tracks))

	publisher := gortsplib.Client{}

	// connect to the server and start publishing
	err = publisher.StartPublishing("rtsp://localhost:8554/mystream2", tracks)
	if err != nil {
		panic(err)
	}
	defer publisher.Close()

	// called when a RTP packet arrives
	reader.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		publisher.WritePacketRTP(ctx.TrackID, ctx.Packet, ctx.PTSEqualsDTS)
	}

	// start reading tracks
	err = reader.SetupAndPlay(tracks, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(reader.Wait())
}
