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

	type trackListenerPair struct {
		rtpl  *gortsplib.ConnClientUdpListener
		rtcpl *gortsplib.ConnClientUdpListener
	}
	var listeners []*trackListenerPair

	for _, track := range tracks {
		rtpl, rtcpl, _, err := conn.SetupUdp(u, track, 9000+track.Id*2, 9001+track.Id*2)
		if err != nil {
			panic(err)
		}

		listeners = append(listeners, &trackListenerPair{
			rtpl:  rtpl,
			rtcpl: rtcpl,
		})
	}

	_, err = conn.Play(u)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup

	for trackId, lp := range listeners {
		wg.Add(2)

		// receive RTP packets
		go func(trackId int, l *gortsplib.ConnClientUdpListener) {
			defer wg.Done()

			buf := make([]byte, 2048)
			for {
				n, err := l.Read(buf)
				if err != nil {
					break
				}

				fmt.Printf("packet from track %d, type RTP: %v\n", trackId, buf[:n])
			}
		}(trackId, lp.rtpl)

		// receive RTCP packets
		go func(trackId int, l *gortsplib.ConnClientUdpListener) {
			defer wg.Done()

			buf := make([]byte, 2048)
			for {
				n, err := l.Read(buf)
				if err != nil {
					break
				}

				fmt.Printf("packet from track %d, type RTCP: %v\n", trackId, buf[:n])
			}
		}(trackId, lp.rtcpl)
	}

	err = conn.LoopUDP(u)
	fmt.Println("connection is closed (%s)", err)

	for _, lp := range listeners {
		lp.rtpl.Close()
		lp.rtcpl.Close()
	}
	wg.Wait()
}
