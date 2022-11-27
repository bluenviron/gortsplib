package main

import (
	"fmt"
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check if there's an H265 track
// 3. get access units of that track

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

	// find the H265 track
	track := func() *gortsplib.TrackH265 {
		for _, track := range tracks {
			if track, ok := track.(*gortsplib.TrackH265); ok {
				return track
			}
		}
		return nil
	}()
	if track == nil {
		panic("H265 track not found")
	}

	// setup RTP/H265->H265 decoder
	dec := track.CreateDecoder()

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// convert RTP packets into NALUs
		nalus, pts, err := dec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		fmt.Println("PTS", pts)

		for _, nalu := range nalus {
			log.Printf("received NALU of size %d\n", len(nalu))
		}
	}

	// setup and read the H265 track only
	err = c.SetupAndPlay(gortsplib.Tracks{track}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
