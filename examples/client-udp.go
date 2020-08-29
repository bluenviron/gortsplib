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

	var rtpReads []gortsplib.UdpReadFunc
	var rtcpReads []gortsplib.UdpReadFunc

	for _, track := range tracks {
		rtpRead, rtcpRead, _, err := conn.SetupUdp(u, track, 9000+track.Id*2, 9001+track.Id*2)
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

		go func(trackId int, rtpRead gortsplib.UdpReadFunc) {
			defer wg.Done()

			buf := make([]byte, 2048)
			for {
				n, err := rtpRead(buf)
				if err != nil {
					break
				}

				fmt.Printf("frame from track %d, type RTP: %v\n", trackId, buf[:n])
			}
		}(trackId, rtpRead)
	}

	// receive RTCP frames
	for trackId, rtcpRead := range rtcpReads {
		wg.Add(1)

		go func(trackId int, rtcpRead gortsplib.UdpReadFunc) {
			defer wg.Done()

			buf := make([]byte, 2048)
			for {
				n, err := rtcpRead(buf)
				if err != nil {
					break
				}

				fmt.Printf("frame from track %d, type RTCP: %v\n", trackId, buf[:n])
			}
		}(trackId, rtcpRead)
	}

	err = conn.LoopUdp(u)
	conn.Close()
	wg.Wait()
	fmt.Println("connection is closed (%s)", err)
}
