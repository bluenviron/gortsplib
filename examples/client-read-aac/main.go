package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/rtpaac"
)

// This example shows how to
// 1. connect to a RTSP server and read all tracks on a path
// 2. check if there's an AAC track
// 3. get AAC AUs of that track

func main() {
	c := gortsplib.Client{}

	// parse URL
	u, err := base.ParseURL("rtsp://localhost:8554/mystream")
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

	// find the AAC track
	var clockRate int
	aacTrack := func() int {
		for i, track := range tracks {
			if _, ok := track.(*gortsplib.TrackAAC); ok {
				clockRate = track.ClockRate()
				return i
			}
		}
		return -1
	}()
	if aacTrack < 0 {
		panic("AAC track not found")
	}

	// setup decoder
	dec := &rtpaac.Decoder{
		SampleRate: clockRate,
	}
	dec.Init()

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		if ctx.TrackID != aacTrack {
			return
		}

		// decode AAC AUs from the RTP packet
		aus, _, err := dec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		// print AUs
		for _, au := range aus {
			log.Printf("received AAC AU of size %d\n", len(au))
		}
	}

	// start reading tracks
	err = c.SetupAndPlay(tracks, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
