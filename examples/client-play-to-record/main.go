package main

import (
	"log"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. read all medias on a path
// 3. re-publish all medias on another path.

func main() {
	reader := gortsplib.Client{}

	// parse source URL
	sourceURL, err := base.ParseURL("rtsp://myuser:mypass@localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	err = reader.Start(sourceURL.Scheme, sourceURL.Host)
	if err != nil {
		panic(err)
	}
	defer reader.Close()

	// find available medias
	desc, _, err := reader.Describe(sourceURL)
	if err != nil {
		panic(err)
	}

	log.Printf("republishing %d medias", len(desc.Medias))

	// setup all medias
	// this must be called before StartRecording(), since it overrides the control attribute.
	err = reader.SetupAll(desc.BaseURL, desc.Medias)
	if err != nil {
		panic(err)
	}

	// connect to the server and start recording the same medias
	publisher := gortsplib.Client{}
	err = publisher.StartRecording("rtsp://myuser:mypass@localhost:8554/mystream2", desc)
	if err != nil {
		panic(err)
	}
	defer publisher.Close()

	// read RTP packets from the reader and route them to the publisher
	reader.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
		publisher.WritePacketRTP(desc.Medias[0], pkt)
	})

	// start playing
	_, err = reader.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(reader.Wait())
}
