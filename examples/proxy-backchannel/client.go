package main

import (
	"log"
	"time"

	"github.com/bluenviron/gortsplib/v5"
	"github.com/bluenviron/gortsplib/v5/pkg/base"
	"github.com/bluenviron/gortsplib/v5/pkg/description"
	"github.com/bluenviron/gortsplib/v5/pkg/format"
	"github.com/pion/rtp"
)

const (
	existingStream = "rtsp://127.0.0.1:8554/mystream"
	reconnectPause = 2 * time.Second
)

func findG711BackChannel(desc *description.Session) (*description.Media, *format.G711) {
	for _, media := range desc.Medias {
		if media.IsBackChannel {
			for _, forma := range media.Formats {
				if g711, ok := forma.(*format.G711); ok {
					return media, g711
				}
			}
		}
	}
	return nil, nil
}

type client struct {
	server *server
}

func (c *client) initialize() {
	// start a separated routine
	go c.run()
}

func (c *client) run() {
	for {
		err := c.read()
		log.Printf("ERR: %s\n", err)

		time.Sleep(reconnectPause)
	}
}

func (c *client) read() error {
	// parse URL
	u, err := base.ParseURL(existingStream)
	if err != nil {
		return err
	}

	rc := gortsplib.Client{
		Scheme:              u.Scheme,
		Host:                u.Host,
		RequestBackChannels: true,
	}

	// connect to the server
	err = rc.Start2()
	if err != nil {
		return err
	}
	defer rc.Close()

	// find available medias
	desc, _, err := rc.Describe(u)
	if err != nil {
		return err
	}

	// find the back channel
	backChannelMedia, _ := findG711BackChannel(desc)
	if backChannelMedia == nil {
		panic("back channel not found")
	}

	writeToClient := func(pkt *rtp.Packet) {
		err2 := rc.WritePacketRTP(backChannelMedia, pkt)
		if err2 != nil {
			log.Printf("ERR: %v", err2)
		}
	}

	// setup all medias
	err = rc.SetupAll(desc.BaseURL, desc.Medias)
	if err != nil {
		return err
	}

	// notify the server that we are ready
	stream := c.server.setStreamReady(desc, writeToClient)
	defer c.server.setStreamUnready()

	log.Printf("stream is ready and can be read from the server at rtsp://localhost:8554/stream\n")

	// called when a RTP packet arrives
	rc.OnPacketRTPAny(func(medi *description.Media, _ format.Format, pkt *rtp.Packet) {
		log.Printf("received RTP packet from the client, routing to readers")

		// route incoming packets to the server stream
		err2 := stream.WritePacketRTP(medi, pkt)
		if err2 != nil {
			log.Printf("ERR: %v", err2)
		}
	})

	// start playing
	_, err = rc.Play(nil)
	if err != nil {
		return err
	}

	// wait until a fatal error
	return rc.Wait()
}
