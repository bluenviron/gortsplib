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

	c := gortsplib.Client{}

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
	err = c.ReadFrames(func(trackID int, streamType gortsplib.StreamType, payload []byte) {
		fmt.Printf("packet from track %d, type %v, size %d\n", trackID, streamType, len(payload))
	})
	panic(err)
}
