package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtpvp8"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's a VP8 media
// 3. get access units of that media

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

	// find available medias
	desc, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// find the VP8 media and format
	var forma *format.VP8
	medi := desc.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// create decoder
	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup a single media
	_, err = c.Setup(desc.BaseURL, medi, 0, 0)
	if err != nil {
		panic(err)
	}

	// called when a RTP packet arrives
	c.OnPacketRTP(medi, forma, func(pkt *rtp.Packet) {
		// decode timestamp
		pts, ok := c.PacketPTS(medi, pkt)
		if !ok {
			log.Printf("waiting for timestamp")
			return
		}

		// extract VP8 frames from RTP packets
		vf, err := rtpDec.Decode(pkt)
		if err != nil {
			if err != rtpvp8.ErrNonStartingPacketAndNoPrevious && err != rtpvp8.ErrMorePacketsNeeded {
				log.Printf("ERR: %v", err)
			}
			return
		}

		log.Printf("received frame with PTS %v size %d\n", pts, len(vf))
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
