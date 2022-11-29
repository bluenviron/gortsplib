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
// 2. check if there's an LPCM track
// 3. get LPCM packets of that track

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

	// find the LPCM media and track
	var trak *track.LPCM
	medi := medias.Find(&trak)
	if medi == nil {
		panic("media not found")
	}

	// setup decoder
	rtpDec := trak.CreateDecoder()

	// called when a RTP packet arrives
	c.OnPacketRTP = func(ctx *gortsplib.ClientOnPacketRTPCtx) {
		// get packets of specific track only
		if ctx.Packet.PayloadType != trak.PayloadType() {
			return
		}

		// decode LPCM samples from the RTP packet
		op, _, err := rtpDec.Decode(ctx.Packet)
		if err != nil {
			return
		}

		// print
		log.Printf("received LPCM samples of size %d\n", len(op))
	}

	// setup and read the LPCM media only
	err = c.SetupAndPlay(media.Medias{medi}, baseURL)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
