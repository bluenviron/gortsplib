package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v3"
	"github.com/bluenviron/gortsplib/v3/pkg/format"
	"github.com/bluenviron/gortsplib/v3/pkg/formatdecenc/rtpvp8"
	"github.com/bluenviron/gortsplib/v3/pkg/url"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's a VP8 media
// 3. get access units of that media

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

	// find the VP8 media and format
	var forma *format.VP8
	medi := medias.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// create decoder
	rtpDec := forma.CreateDecoder()

	// setup a single media
	_, err = c.Setup(medi, baseURL, 0, 0)
	if err != nil {
		panic(err)
	}

	// called when a RTP packet arrives
	c.OnPacketRTP(medi, forma, func(pkt *rtp.Packet) {
		// extract VP8 frames from RTP packets
		vf, _, err := rtpDec.Decode(pkt)
		if err != nil {
			if err != rtpvp8.ErrNonStartingPacketAndNoPrevious && err != rtpvp8.ErrMorePacketsNeeded {
				log.Printf("ERR: %v", err)
			}
			return
		}

		log.Printf("received frame of size %d\n", len(vf))
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
