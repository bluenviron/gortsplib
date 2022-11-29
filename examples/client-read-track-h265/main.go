package main

import (
	"log"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/media"
	"github.com/aler9/gortsplib/pkg/track"
	"github.com/aler9/gortsplib/pkg/url"
)

// This example shows how to
// 1. connect to a RTSP server
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

	// find published medias
	medias, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the H265 media and track
	var trak *track.H265
	medi := medias.Find(&trak)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP/H265->H265 decoder
	rtpDec := trak.CreateDecoder()

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// get packets of specific track only
		if ctx.Packet.PayloadType != trak.PayloadType() {
			return
		}

		// convert RTP packets into NALUs
		nalus, pts, err := rtpDec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		for _, nalu := range nalus {
			log.Printf("received NALU with PTS %v and size %d\n", pts, len(nalu))
		}
	}

	// setup and read the H265 media only
	err = c.SetupAndPlay(media.Medias{medi}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
