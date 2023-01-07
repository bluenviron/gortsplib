package main

import (
	"bytes"
	"image/jpeg"
	"log"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/formatdecenc/rtpmjpeg"
	"github.com/aler9/gortsplib/v2/pkg/url"
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

	// find the M-JPEG media and format
	var forma *format.MJPEG
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
		// extract JPEG images from RTP packets
		enc, pts, err := rtpDec.Decode(pkt)
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

		log.Printf("decoded image with size %v and pts %v", image.Bounds().Max, pts)
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
