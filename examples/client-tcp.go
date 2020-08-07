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
		_, err := conn.SetupTcp(u, track)
		if err != nil {
			panic(err)
		}
	}

	_, err = conn.Play(u)
	if err != nil {
		panic(err)
	}

	frame := &gortsplib.InterleavedFrame{Content: make([]byte, 0, 128*1024)}

	for {
		frame.Content = frame.Content[:cap(frame.Content)]

		err := conn.ReadFrame(frame)
		if err != nil {
			fmt.Println("connection is closed (%s)", err)
			break
		}

		fmt.Printf("packet from track %d, type %v: %v\n",
			frame.TrackId, frame.StreamType, frame.Content)
	}
}
