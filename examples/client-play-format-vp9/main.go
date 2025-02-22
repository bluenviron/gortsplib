//go:build cgo

package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtpvp9"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's an VP9 format
// 3. decode the VP9 stream into RGBA frames

// This example requires the FFmpeg libraries, that can be installed with this command:
// apt install -y libavcodec-dev libswscale-dev gcc pkg-config

func main() {
	c := gortsplib.Client{}

	// parse URL
	u, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/mystream")
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

	// find the VP9 media and format
	var forma *format.VP9
	medi := desc.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP -> VP9 decoder
	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup VP9 -> RGBA decoder
	vp9Dec := &vp9Decoder{}
	err = vp9Dec.initialize()
	if err != nil {
		panic(err)
	}
	defer vp9Dec.close()

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
			if err != rtpvp9.ErrNonStartingPacketAndNoPrevious && err != rtpvp9.ErrMorePacketsNeeded {
				log.Printf("ERR: %v", err)
			}
			return
		}

		// convert VP9 access units into RGBA frames
		img, err := vp9Dec.decode(au)
		if err != nil {
			panic(err)
		}

		// wait for a frame
		if img == nil {
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
