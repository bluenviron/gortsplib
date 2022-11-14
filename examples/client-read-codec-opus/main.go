package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtpopus"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check if there's an Opus track
// 3. get Opus packets of that track

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

	// find published tracks
	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the Opus track
	track := func() *gortsplib.TrackOpus {
		for _, track := range tracks {
			if tt, ok := track.(*gortsplib.TrackOpus); ok {
				return tt
			}
		}
		return nil
	}()
	if track == nil {
		panic("Opus track not found")
	}

	// setup decoder
	dec := &rtpopus.Decoder{
		SampleRate: track.SampleRate,
	}
	dec.Init()

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// decode an Opus packet from the RTP packet
		op, _, err := dec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		// print
		log.Printf("received Opus packet of size %d\n", len(op))
	}

	// setup and read the Opus track only
	err = c.SetupAndPlay(gortsplib.Tracks{track}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
