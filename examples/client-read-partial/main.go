package main

import (
	"fmt"

	"github.com/aler9/gortsplib"
	"github.com/aler9/gortsplib/pkg/base"
)

// This example shows how to
// 1. connect to a RTSP server
// 2. get tracks published on a path
// 3. read only selected tracks

func main() {
	c := gortsplib.Client{
		// called when a RTP packet arrives
		OnPacketRTP: func(trackID int, payload []byte) {
			fmt.Printf("RTP packet from track %d, size %d\n", trackID, len(payload))
		},
		// called when a RTCP packet arrives
		OnPacketRTCP: func(trackID int, payload []byte) {
			fmt.Printf("RTCP packet from track %d, size %d\n", trackID, len(payload))
		},
	}

	u, err := base.ParseURL("rtsp://myserver/mypath")
	if err != nil {
		panic(err)
	}

	err = c.Start(u.Scheme, u.Host)
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

	// setup only H264 tracks, skipping audio or application tracks
	for _, t := range tracks {
		if _, ok := t.(*gortsplib.TrackH264); ok {
			_, err := c.Setup(true, t, baseURL, 0, 0)
			if err != nil {
				panic(err)
			}
		}
	}

	// start reading tracks
	_, err = c.Play(nil)
	if err != nil {
		panic(err)
	}

	// wait until a fatal error
	panic(c.Wait())
}
