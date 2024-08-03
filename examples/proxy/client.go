package main

import (
	"log"
	"time"

	"github.com/bluenviron/gortsplib/v4"
	"github.com/bluenviron/gortsplib/v4/pkg/base"
	"github.com/bluenviron/gortsplib/v4/pkg/description"
	"github.com/bluenviron/gortsplib/v4/pkg/format"
	"github.com/pion/rtp"
)

const (
	existingStream = "rtsp://u37001:p37001@192.168.26.202/onvif-media/media.amp?profile=profile_1_h264&sessiontimeout=60&streamtype=unicast"
	reconnectPause = 2 * time.Second
)

type client struct {
	s *server
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
	rc := gortsplib.Client{}

	// parse URL
	u, err := base.ParseURL(existingStream)
	if err != nil {
		return err
	}

	// connect to the server
	err = rc.Start(u.Scheme, u.Host)
	if err != nil {
		return err
	}
	defer rc.Close()

	// find available medias
	desc, _, err := rc.Describe(u)
	if err != nil {
		return err
	}

	// setup all medias
	err = rc.SetupAll(desc.BaseURL, desc.Medias)
	if err != nil {
		return err
	}

	// HACK!!!
	// HACK. Add backchannel into desc before it is passed to the Server with setStreamReady()
	// HACK!!!
	var extraMedia = new(description.Media)
	extraMedia.IsBackChannel = true
	extraMedia.Type = description.MediaTypeAudio
	extraMedia.Control = "track=extrabackchannel"
	extraMedia.Formats = []format.Format{&format.G711{
		PayloadTyp:   0, // 0 = MULAW 8 = ALAW
		MULaw:        true,
		SampleRate:   8000,
		ChannelCount: 1,
	}}

	desc.Medias = append(desc.Medias, extraMedia)

	stream := c.s.setStreamReady(desc)
	defer c.s.setStreamUnready()

	log.Printf("stream is ready and can be read from the server at rtsp://localhost:8554/stream\n")

	// called when a RTP packet arrives
	rc.OnPacketRTPAny(func(medi *description.Media, forma format.Format, pkt *rtp.Packet) {
		// route incoming packets to the server stream
		stream.WritePacketRTP(medi, pkt)
	})

	// start playing
	_, err = rc.Play(nil)
	if err != nil {
		return err
	}

	// wait until a fatal error
	return rc.Wait()
}
