package main

import (
	"bytes"
	"image/jpeg"
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtpmjpeg"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's a M-JPEG media
// 3. get JPEG images of that media
// 4. decode JPEG images into raw images

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

	// find the M-JPEG media and format
	var forma *format.MJPEG
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

		// extract JPEG images from RTP packets
		enc, err := rtpDec.Decode(pkt)
		if err != nil {
			if err != rtpmjpeg.ErrNonStartingPacketAndNoPrevious && err != rtpmjpeg.ErrMorePacketsNeeded {
				log.Printf("ERR: %v", err)
			}
			return
		}

		// convert JPEG images into raw images
		image, err := jpeg.Decode(bytes.NewReader(enc))
		if err != nil {
			panic(err)
		}

		log.Printf("decoded image with PTS %v and size %v", pts, image.Bounds().Max)
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
