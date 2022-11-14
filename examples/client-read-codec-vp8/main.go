package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/rtpvp8"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check if there's an VP8 track
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

	// find the VP8 track
	vp8Track, vp8TrackID := func() (*gortsplib.TrackVP8, int) {
		for i, track := range tracks {
			if tt, ok := track.(*gortsplib.TrackVP8); ok {
				return tt, i
			}
		}
		return nil, -1
	}()
	if vp8Track == nil {
		panic("VP8 track not found")
	}

	// setup decoder
	dec := &rtpvp8.Decoder{}
	dec.Init()

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		if ctx.TrackID != vp8TrackID {
			return
		}

		// decode a VP8 frame from the RTP packet
		vf, _, err := dec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		log.Printf("received frame of size %d\n", len(vf))
	}

	// setup and read all tracks
	err = c.SetupAndPlay(tracks, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
