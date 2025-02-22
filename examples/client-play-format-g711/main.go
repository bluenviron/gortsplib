package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/bluenviron/mediacommon/v2/pkg/codecs/g711"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. check if there's a G711 stream
// 3. decode the G711 stream into audio samples

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

	// find the G711 media and format
	var forma *format.G711
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

		// extract G711 samples from RTP packets
		samples, err := rtpDec.Decode(pkt)
		if err != nil {
			log.Printf("ERR: %v", err)
			return
		}

		// decode samples (these are 16-bit, big endian LPCM samples)
		if forma.MULaw {
			var raw g711.Mulaw
			raw.Unmarshal(samples)
			samples = raw
		} else {
			var raw g711.Alaw
			raw.Unmarshal(samples)
			samples = raw
		}

		// print
		log.Printf("decoded audio samples with PTS %v and size %d\n", pts, len(samples))
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
