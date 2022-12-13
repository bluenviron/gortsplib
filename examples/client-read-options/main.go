package main

import (
	"log"
	"time"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/aler9/gortsplib/v2/pkg/url"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. set additional client options
// 2. connect to a RTSP server and read all medias on a path

func main() {
	// Client allows to set additional client options
	c := &gortsplib.Client{
		// the stream transport (UDP, Multicast or TCP). If nil, it is chosen automatically
		Transport: nil,
		// timeout of read operations
		ReadTimeout: 10 * time.Second,
		// timeout of write operations
		WriteTimeout: 10 * time.Second,
	}

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

	// setup all medias
	err = c.SetupAll(medias, baseURL)
	if err != nil {
		panic(err)
	}

	// called when a RTP packet arrives
	c.OnPacketRTPAny(func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
		log.Printf("RTP packet from media %v\n", medi)
	})

	// called when a RTCP packet arrives
	c.OnPacketRTCPAny(func(medi *media.Media, pkt rtcp.Packet) {
		log.Printf("RTCP packet from media %v, type %T\n", medi, pkt)
	})

	// start playing
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
