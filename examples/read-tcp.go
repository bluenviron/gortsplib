// +build ignore

package main

import (
	"fmt"
	"net/url"

	"github.com/aler9/gortsplib"
)

func main() {
	u, err := url.Parse("rtsp://user:pass@example.com/mystream")
	if err != nil {
		panic(err)
	}

	conn, err := gortsplib.NewConnClient(gortsplib.ConnClientConf{Host: u.Host})
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

	for _, track := range tracks {
		_, err := conn.SetupTCP(u, gortsplib.SetupModePlay, track)
		if err != nil {
			panic(err)
		}
	}

	_, err = conn.Play(u)
	if err != nil {
		panic(err)
	}

	for {
		frame, err := conn.ReadFrameTCP()
		if err != nil {
			fmt.Println("connection is closed (%s)", err)
			break
		}

		fmt.Printf("frame from track %d, type %v: %v\n",
			frame.TrackId, frame.StreamType, frame.Content)
	}
}
