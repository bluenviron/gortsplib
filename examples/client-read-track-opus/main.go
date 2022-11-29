package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/track"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's an Opus track
// 3. get Opus packets of that track

func findTrack(medias gortsplib.Medias) (*gortsplib.Media, *track.Opus) {
	for _, media := range medias {
		for _, trak := range media.Tracks {
			if trak, ok := trak.(*track.Opus); ok {
				return media, trak
			}
		}
	}
	return nil, nil
}

func main() {
	c := gortsplib.Client{}

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

	// find the Opus media and track
	media, track := findTrack(medias)
	if media == nil {
		panic("media not found")
	}

	// setup decoder
	dec := track.CreateDecoder()

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// get packets of specific track only
		if ctx.Packet.PayloadType != track.PayloadType() {
			return
		}

		// decode an Opus packet from the RTP packet
		op, _, err := dec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		// print
		log.Printf("received Opus packet of size %d\n", len(op))
	}

	// setup and read the Opus media only
	err = c.SetupAndPlay(gortsplib.Medias{media}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
