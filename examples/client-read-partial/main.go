package main

import (
	"fmt"
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. get tracks published on a path
// 3. read only the H264 track

func main() {
	c := gortsplib.Client{
		// called when a RTP packet arrives
		OnPacketRTP: func(ctx *gortsplib.ClientOnPacketRTPCtx) {
			log.Printf("RTP packet from track %d, payload type %d\n", ctx.TrackID, ctx.Packet.Header.PayloadType)
		},
		// called when a RTCP packet arrives
		OnPacketRTCP: func(ctx *gortsplib.ClientOnPacketRTCPCtx) {
			log.Printf("RTCP packet from track %d, type %T\n", ctx.TrackID, ctx.Packet)
		},
	}

	u, err := url.Parse("rtsp://myserver/mypath")
	if err != nil {
		panic(err)
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the H264 track
	h264Track := func() gortsplib.Track {
		for _, t := range tracks {
			if _, ok := t.(*gortsplib.TrackH264); ok {
				return t
			}
		}
		return nil
	}()
	if h264Track == nil {
		panic(fmt.Errorf("H264 track not found"))
	}

	// setup and play the H264 track only
	err = c.SetupAndPlay(gortsplib.Tracks{h264Track}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
