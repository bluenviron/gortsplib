// Package main contains an example.
package main

import (
	"bytes"
	"errors"
	"image/jpeg"
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtpmjpeg"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server.
// 2. check if there's a M-JPEG stream.
// 3. get JPEG images of that format.
// 4. decode JPEG images into RGBA frames.

func main() {
	// parse URL
	u, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	c := gortsplib.Client{
		Scheme: u.Scheme,
		Host:   u.Host,
	}

	// connect to the server
	err = c.Start2()
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
		pts, ok := c.PacketPTS2(medi, pkt)
		if !ok {
			log.Printf("waiting for timestamp")
			return
		}

		// extract JPEG images from RTP packets
		enc, err2 := rtpDec.Decode(pkt)
		if err2 != nil {
			if !errors.Is(err2, rtpmjpeg.ErrNonStartingPacketAndNoPrevious) && !errors.Is(err2, rtpmjpeg.ErrMorePacketsNeeded) {
				log.Printf("ERR: %v", err2)
			}
			return
		}

		// convert JPEG images into RGBA frames
		image, err2 := jpeg.Decode(bytes.NewReader(enc))
		if err2 != nil {
			panic(err2)
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
