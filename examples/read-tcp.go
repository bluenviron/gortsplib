// +build ignore

package main

import (
	"fmt"
	"net/url"

	"github.com/aler9/gortsplib"
)

func main() {
	// parse url
	u, err := url.Parse("rtsp://localhost:8554/mystream")
	if err != nil {
		panic(err)
	}

	// connect to the server
	conn, err := gortsplib.NewConnClient(gortsplib.ConnClientConf{Host: u.Host})
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// get allowed commands
	_, err = conn.Options(u)
	if err != nil {
		panic(err)
	}

	// list tracks published on the path
	tracks, _, err := conn.Describe(u)
	if err != nil {
		panic(err)
	}

	// setup tracks with TCP
	for _, track := range tracks {
		_, err := conn.SetupTCP(u, gortsplib.SetupModePlay, track)
		if err != nil {
			panic(err)
		}
	}

	// start reading
	_, err = conn.Play(u)
	if err != nil {
		panic(err)
	}

	for {
		// read frames
		frame, err := conn.ReadFrameTCP()
		if err != nil {
			fmt.Println("connection is closed (%s)", err)
			break
		}

		fmt.Printf("frame from track %d, type %v: %v\n",
			frame.TrackId, frame.StreamType, frame.Content)
	}
}
