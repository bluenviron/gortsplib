package main

import (
	"log"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/url"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's an MPEG4-audio media
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

	// find the MPEG4-audio media and format
	var forma *format.MPEG4Audio
	medi := medias.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// setup decoder
	rtpDec := forma.CreateDecoder()

	// setup the chosen media only
	_, err = c.Setup(medi, baseURL, 0, 0)
	if err != nil {
		panic(err)
	}

	// called when a RTP packet arrives
	c.OnPacketRTP(medi, forma, func(pkt *rtp.Packet) {
		// decode MPEG4-audio AUs from the RTP packet
		aus, _, err := rtpDec.Decode(pkt)
		if err != nil {
			return
		}

		// print AUs
		for _, au := range aus {
			log.Printf("received MPEG4-audio AU of size %d\n", len(au))
		}
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
