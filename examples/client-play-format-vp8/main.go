//go:build cgo

// Package main contains an example.
package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/bluenviron/gortsplib/v5/pkg/format/rtpvp8"
	"github.com/pion/rtp"
)

// This example shows how to:
// 1. connect to a RTSP server.
// 2. check if there's a VP8 stream.
// 3. decode the VP8 stream into RGBA frames.

// This example requires the FFmpeg libraries, that can be installed with this command:
// apt install -y libavcodec-dev libswscale-dev gcc pkg-config

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

	// find the VP8 media and format
	var forma *format.VP8
	medi := desc.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP -> VP8 decoder
	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup VP8 -> RGBA decoder
	vp8Dec := &vp8Decoder{}
	err = vp8Dec.initialize()
	if err != nil {
		panic(err)
	}
	defer vp8Dec.close()

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

		// extract access units from RTP packets
		au, err := rtpDec.Decode(pkt)
		if err != nil {
			if err != rtpvp8.ErrNonStartingPacketAndNoPrevious && err != rtpvp8.ErrMorePacketsNeeded {
				log.Printf("ERR: %v", err)
			}
			return
		}

		// convert VP8 access units into RGBA frames
		img, err := vp8Dec.decode(au)
		if err != nil {
			panic(err)
		}

		// check for frame presence
		if img == nil {
			log.Printf("ERR: frame cannot be decoded")
			return
		}

		log.Printf("decoded frame with PTS %v and size %v", pts, img.Bounds().Max)
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
