package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
	"github.com/aler9/gortsplib/pkg/headers"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. get tracks published on a path
// 3. read only selected tracks

func main() {
	u, err := base.ParseURL("rtsp://myserver/mypath")
	if err != nil {
		panic(err)
	}

	c := gortsplib.Client{
		// called when a RTP packet arrives
		OnPacketRTP: func(c *gortsplib.Client, trackID int, payload []byte) {
			fmt.Printf("RTP packet from track %d, size %d\n", trackID, len(payload))
		},
		// called when a RTCP packet arrives
		OnPacketRTCP: func(c *gortsplib.Client, trackID int, payload []byte) {
			fmt.Printf("RTCP packet from track %d, size %d\n", trackID, len(payload))
		},
	}

	err = c.Dial(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	_, err = c.Options(u)
	if err != nil {
		panic(err)
	}

	tracks, baseURL, _, err := c.Describe(u)
	if err != nil {
		panic(err)
	}

	// start reading only video tracks, skipping audio or application tracks
	for _, t := range tracks {
		if t.Media.MediaName.Media == "video" {
			_, err := c.Setup(headers.TransportModePlay, baseURL, t, 0, 0)
			if err != nil {
				panic(err)
			}
		}
	}

	// play setupped tracks
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// read packets
	err = c.ReadFrames()
	panic(err)
}
