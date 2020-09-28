// +build ignore

package main

import (
	"fmt"
	"net/url"
	"sync"

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

	// setup tracks with UDP
	for _, track := range tracks {
		_, err := conn.SetupUDP(u, gortsplib.SetupModePlay, track, 0, 0)
		if err != nil {
			panic(err)
		}
	}

	// start reading
	_, err = conn.Play(u)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()
	defer conn.CloseUDPListeners()

	for _, track := range tracks {
		// read RTP frames
		wg.Add(1)
		go func(track *gortsplib.Track) {
			defer wg.Done()

			for {
				buf, err := conn.ReadFrameUDP(track, gortsplib.StreamTypeRtp)
				if err != nil {
					break
				}

				fmt.Printf("frame from track %d, type RTP: %v\n", track.Id, buf)
			}
		}(track)

		// read RTCP frames
		wg.Add(1)
		go func(track *gortsplib.Track) {
			defer wg.Done()

			for {
				buf, err := conn.ReadFrameUDP(track, gortsplib.StreamTypeRtcp)
				if err != nil {
					break
				}

				fmt.Printf("frame from track %d, type RTCP: %v\n", track.Id, buf)
			}
		}(track)
	}

	err = conn.LoopUDP(u)
	fmt.Println("connection is closed (%s)", err)
}
