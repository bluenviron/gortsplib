package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check if there's an MPEG4-audio track
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

	// find the MPEG4-audio track
	track := func() *gortsplib.TrackMPEG4Audio {
		for _, track := range tracks {
			if tt, ok := track.(*gortsplib.TrackMPEG4Audio); ok {
				return tt
			}
		}
		return nil
	}()
	if track == nil {
		panic("MPEG4-audio track not found")
	}

	// setup decoder
	dec := track.CreateDecoder()

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// decode MPEG4-audio AUs from the RTP packet
		aus, _, err := dec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		// print AUs
		for _, au := range aus {
			log.Printf("received MPEG4-audio AU of size %d\n", len(au))
		}
	}

	// setup and read the MPEG4-audio track only
	err = c.SetupAndPlay(gortsplib.Tracks{track}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
