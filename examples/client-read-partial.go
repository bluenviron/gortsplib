// +build ignore

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

	conn, err := gortsplib.Dial(u.Scheme, u.Host)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	_, err = conn.Options(u)
	if err != nil {
		panic(err)
	}

	tracks, _, err := conn.Describe(u)
	if err != nil {
		panic(err)
	}

	for _, t := range tracks {
		// start reading only video tracks, skipping audio or application tracks
		if t.Media.MediaName.Media == "video" {
			_, err := conn.Setup(headers.TransportModePlay, t, 0, 0)
			if err != nil {
				panic(err)
			}
		}
	}

	_, err = conn.Play()
	if err != nil {
		panic(err)
	}

	// read track frames
	err = <-conn.ReadFrames(func(trackID int, typ gortsplib.StreamType, buf []byte) {
		fmt.Printf("frame from track %d, type %v: %v\n", trackID, typ, buf)
	})
	panic(err)
}
