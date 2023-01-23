package main

import (
	"log"
	"sync"
	"time"

	"github.com/aler9/gortsplib/v2"
	"github.com/aler9/gortsplib/v2/pkg/format"
	"github.com/aler9/gortsplib/v2/pkg/media"
	"github.com/aler9/gortsplib/v2/pkg/url"
	"github.com/pion/rtp"
)

const (
	existingStream = "rtsp://x.x.x.x:8554/mystream"
	reconnectPause = 2 * time.Second
)

type client struct {
	mutex  sync.RWMutex
	stream *gortsplib.ServerStream
}

func newClient() *client {
	c := &client{}

	// start a separated routine
	go c.run()

	return c
}

func (c *client) run() {
	for {
		err := c.read()
		log.Printf("ERR: %s\n", err)

		time.Sleep(reconnectPause)
	}
}

func (c *client) getStream() *gortsplib.ServerStream {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.stream
}

func (c *client) read() error {
	rc := gortsplib.Client{}

	// parse URL
	u, err := url.Parse(existingStream)
	if err != nil {
		return err
	}

	// connect to the server
	err = rc.Start(u.Scheme, u.Host)
	if err != nil {
		return err
	}
	defer rc.Close()

	// find published medias
	medias, baseURL, _, err := rc.Describe(u)
	if err != nil {
		return err
	}

	// setup all medias
	err = rc.SetupAll(medias, baseURL)
	if err != nil {
		return err
	}

	// create a server stream
	stream := gortsplib.NewServerStream(medias)
	defer stream.Close()

	log.Printf("stream is ready and can be read from the server at rtsp://localhost:8554/stream\n")

	// make stream available by using getStream()
	c.mutex.Lock()
	c.stream = stream
	c.mutex.Unlock()

	defer func() {
		// remove stream from getStream()
		c.mutex.Lock()
		c.stream = nil
		c.mutex.Unlock()
	}()

	// called when a RTP packet arrives
	rc.OnPacketRTPAny(func(medi *media.Media, forma format.Format, pkt *rtp.Packet) {
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
