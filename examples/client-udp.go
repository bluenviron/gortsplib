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

	var rtpReads []gortsplib.UDPReadFunc
	var rtcpReads []gortsplib.UDPReadFunc

	for _, track := range tracks {
		rtpRead, rtcpRead, _, err := conn.SetupUDP(u, track, 9000+track.Id*2, 9001+track.Id*2)
		if err != nil {
			panic(err)
		}

		rtpReads = append(rtpReads, rtpRead)
		rtcpReads = append(rtcpReads, rtcpRead)
	}

	_, err = conn.Play(u)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	// receive RTP frames
	for trackId, rtpRead := range rtpReads {
		wg.Add(1)

		go func(trackId int, rtpRead gortsplib.UDPReadFunc) {
			defer wg.Done()

			for {
				buf, err := rtpRead()
				if err != nil {
					break
				}

				fmt.Printf("frame from track %d, type RTP: %v\n", trackId, buf)
			}
		}(trackId, rtpRead)
	}

	// receive RTCP frames
	for trackId, rtcpRead := range rtcpReads {
		wg.Add(1)

		go func(trackId int, rtcpRead gortsplib.UDPReadFunc) {
			defer wg.Done()

			for {
				buf, err := rtcpRead()
				if err != nil {
					break
				}

				fmt.Printf("frame from track %d, type RTCP: %v\n", trackId, buf)
			}
		}(trackId, rtcpRead)
	}

	err = conn.LoopUDP(u)
	conn.Close()
	wg.Wait()
	fmt.Println("connection is closed (%s)", err)
}
