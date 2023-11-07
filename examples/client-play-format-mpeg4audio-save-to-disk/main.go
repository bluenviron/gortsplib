package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's an MPEG-4 audio media
// 3. save the content of the media into a file in MPEG-TS format

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

	// find the MPEG-4 audio media and format
	var forma *format.MPEG4Audio
	medi := desc.FindFormat(&forma)
	if medi == nil {
		panic("media not found")
	}

	// setup RTP/MPEG-4 audio -> MPEG-4 audio decoder
	rtpDec, err := forma.CreateDecoder()
	if err != nil {
		panic(err)
	}

	// setup MPEG-4 audio -> MPEG-TS muxer
	mpegtsMuxer, err := newMPEGTSMuxer(forma.Config)
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

		// extract access units from RTP packets
		aus, err := rtpDec.Decode(pkt)
		if err != nil {
			log.Printf("ERR: %v", err)
			return
		}

		// encode access units into MPEG-TS
		err = mpegtsMuxer.encode(aus, pts)
		if err != nil {
			log.Printf("ERR: %v", err)
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
