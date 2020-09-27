// +build ignore

package main

import (
	"fmt"
	"net/url"
	"sync"

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
		_, err := conn.SetupUDP(u, gortsplib.SetupModePlay, track, 0, 0)
		if err != nil {
			panic(err)
		}
	}

	_, err = conn.Play(u)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	defer wg.Wait()
	defer conn.CloseUDPListeners()

	// read RTP frames
	for _, track := range tracks {
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
	}

	// read RTCP frames
	for _, track := range tracks {
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
