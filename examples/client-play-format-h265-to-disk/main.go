// Package main contains an example.
package main

import (
	"errors"
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/gortsplib/v4/pkg/format/rtph265"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server.
// 2. check if there's a H265 stream.
// 3. save the content of the format in a file in MPEG-TS format.

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

	// find the H265 media and format
	var forma *format.H265
	medi := desc.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP -> H265 decoder
	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup H265 -> MPEG-TS muxer
	mpegtsMuxer := &mpegtsMuxer{
		fileName: "mystream.ts",
		vps:      forma.VPS,
		sps:      forma.SPS,
		pps:      forma.PPS,
	}
	err = mpegtsMuxer.initialize()
	if err != nil {
		panic(err)
	}
	defer mpegtsMuxer.close()

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

		// extract access unit from RTP packets
		au, err2 := rtpDec.Decode(pkt)
		if err2 != nil {
			if !errors.Is(err2, rtph265.ErrNonStartingPacketAndNoPrevious) && !errors.Is(err2, rtph265.ErrMorePacketsNeeded) {
				log.Printf("ERR: %v", err2)
			}
			return
		}

		// encode the access unit into MPEG-TS
		err2 = mpegtsMuxer.writeH265(au, pts)
		if err2 != nil {
			log.Printf("ERR: %v", err2)
			return
		}

		log.Printf("saved TS packet")
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
