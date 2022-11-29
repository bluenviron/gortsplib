package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/track"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. get tracks published on a path
// 3. read only the H264 track

func findTrack(medias gortsplib.Medias) (*gortsplib.Media, *track.H264) {
	for _, media := range medias {
		for _, trak := range media.Tracks {
			if trak, ok := trak.(*track.H264); ok {
				return media, trak
			}
		}
	}
	return nil, nil
}

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

	u, err := url.Parse("rtsp://myserver/mypath")
	if err != nil {
		panic(err)
	}

	err = c.Start(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	medias, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the H264 media and track
	media, _ := findTrack(medias)
	if media == nil {
		panic("media not found")
	}

	// setup and play the H264 media only
	err = c.SetupAndPlay(gortsplib.Medias{media}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
