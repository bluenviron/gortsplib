// Package main contains an example.
package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/formats/mpegts"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server.
// 2. check if there's a MPEG-4 audio stream.
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

	// find the MPEG-4 audio media and format
	var forma *format.MPEG4Audio
	medi := desc.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP -> MPEG-4 audio decoder
	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup MPEG-4 audio -> MPEG-TS muxer
	mpegtsMuxer := &mpegtsMuxer{
		fileName: "mystream.ts",
		format:   forma,
		track: &mpegts.Track{
			Codec: &mpegts.CodecMPEG4Audio{
				Config: *forma.Config,
			},
		},
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

		// extract access units from RTP packets
		aus, err2 := rtpDec.Decode(pkt)
		if err2 != nil {
			log.Printf("ERR: %v", err2)
			return
		}

		// encode access units into MPEG-TS
		err2 = mpegtsMuxer.writeMPEG4Audio(aus, pts)
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
